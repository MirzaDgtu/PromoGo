package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// queuedOp is one operation the register could not deliver. Only sales and
// refunds are queued: both are facts that already happened at the till and
// must reach the loyalty API eventually. A redemption is never queued — the
// customer is paying with points right now, and spending a balance nobody
// can check is exactly what offline mode must not allow.
type queuedOp struct {
	Kind     string         `json:"kind"` // "sale" or "refund"
	Sale     *saleRequest   `json:"sale,omitempty"`
	Refund   *refundRequest `json:"refund,omitempty"`
	QueuedAt time.Time      `json:"queued_at"`
	LastErr  string         `json:"last_error,omitempty"`
}

func (op queuedOp) transactionID() string {
	if op.Sale != nil {
		return op.Sale.TransactionID
	}
	if op.Refund != nil {
		return op.Refund.TransactionID
	}
	return ""
}

// queue is the register's local outbox: a JSON-lines file, one operation
// per line, in the order the sales happened.
type queue struct{ path string }

func (q queue) load() ([]queuedOp, error) {
	f, err := os.Open(q.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ops []queuedOp
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for line := 1; sc.Scan(); line++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var op queuedOp
		if err := json.Unmarshal(sc.Bytes(), &op); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", q.path, line, err)
		}
		ops = append(ops, op)
	}
	return ops, sc.Err()
}

func (q queue) append(op queuedOp) error {
	f, err := os.OpenFile(q.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeOp(f, op)
}

// save rewrites the queue with what is still undelivered, removing the
// file once it is empty. It writes a temp file and renames it over the
// queue, so a crash mid-replay never loses or duplicates an operation.
func (q queue) save(ops []queuedOp) error {
	if len(ops) == 0 {
		if err := os.Remove(q.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	tmp := q.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if err := writeOp(f, op); err != nil {
			f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, q.path)
}

func writeOp(w io.Writer, op queuedOp) error {
	line, err := json.Marshal(op)
	if err != nil {
		return err
	}
	_, err = w.Write(append(line, '\n'))
	return err
}

// replayResult tallies one replay pass.
type replayResult struct {
	Delivered, Replayed, Rejected, Left int
}

// replay sends the queue in order. It stops at the first transient failure
// so later sales never overtake an earlier one; a request the API rejects
// for good (4xx) is reported and dropped, because resending it can never
// succeed. Every delivery reuses the original transaction_id, so an
// operation the API already received before the link dropped comes back
// as replayed instead of being counted twice.
func replay(ctx context.Context, c *client, q queue, report func(op queuedOp, outcome string)) (replayResult, error) {
	ops, err := q.load()
	if err != nil {
		return replayResult{}, err
	}

	var res replayResult
	for i, op := range ops {
		replayed, err := deliver(ctx, c, op)
		switch {
		case err == nil:
			if replayed {
				res.Replayed++
				report(op, "уже было принято ранее (replayed)")
			} else {
				res.Delivered++
				report(op, "доставлено")
			}
		case isTransient(err):
			rest := ops[i:]
			rest[0].LastErr = err.Error()
			res.Left = len(rest)
			if saveErr := q.save(rest); saveErr != nil {
				return res, saveErr
			}
			return res, err
		default:
			res.Rejected++
			report(op, "отклонено: "+err.Error())
		}
	}
	return res, q.save(nil)
}

func deliver(ctx context.Context, c *client, op queuedOp) (replayed bool, err error) {
	switch {
	case op.Sale != nil:
		resp, err := c.sale(ctx, *op.Sale)
		return resp.Replayed, err
	case op.Refund != nil:
		resp, err := c.refund(ctx, *op.Refund)
		return resp.Replayed, err
	default:
		return false, &apiError{Status: 0, Message: "пустая запись в очереди (kind=" + op.Kind + ")"}
	}
}
