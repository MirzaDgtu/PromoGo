// Command possim is a register (1C/POS) simulator for PromoGo's store-key
// API. It sends sales, redemptions and refunds exactly as a 1C cash
// register module would — store API key, client-chosen transaction_id,
// retries that reuse that id — and keeps an offline queue that replays on
// reconnect, so the loyalty API can be exercised end to end before a real
// 1C base exists. It is also the reference for whoever writes that module.
//
// It only speaks HTTP: nothing here imports the server's packages.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shopspring/decimal"
)

const usage = `possim — эмулятор кассы/1С для PromoGo

использование:
  possim <команда> [флаги]

команды:
  sale     продажа: начислить баллы          -phone -amount [-id]
  redeem   оплата баллами                    -phone -points -amount [-id]
  refund   возврат по продаже или списанию   -original -amount [-type] [-id]
  balance  баланс клиента по телефону        -phone
  qr       опознать клиента по QR из приложения  -payload
  flood    поток продаж от разных клиентов   [-n] [-clients] [-redeem-share]
  replay   досылка офлайн-очереди
  queue    показать офлайн-очередь

общие флаги (или переменные окружения):
  -url    адрес API      (PROMOGO_URL, по умолчанию http://127.0.0.1:8090)
  -key    API-ключ магазина (PROMOGO_STORE_KEY)
  -queue  файл офлайн-очереди (PROMOGO_QUEUE, по умолчанию possim-queue.jsonl)

У sale, refund и flood есть -offline: связи «нет», операции сразу уходят в
очередь. Перед новой продажей очередь досылается сама, как при восстановлении
связи у настоящей кассы.
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "help" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmds := map[string]func(context.Context, []string) error{
		"sale":    runSale,
		"redeem":  runRedeem,
		"refund":  runRefund,
		"balance": runBalance,
		"qr":      runQR,
		"flood":   runFlood,
		"replay":  runReplay,
		"queue":   runQueue,
	}
	run, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "неизвестная команда %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err := run(ctx, os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

// env holds what every command needs: the API client and the queue.
type env struct {
	client *client
	queue  queue
}

type commonFlags struct {
	url, key, queue *string
}

func addCommon(fs *flag.FlagSet) commonFlags {
	return commonFlags{
		url:   fs.String("url", envOr("PROMOGO_URL", "http://127.0.0.1:8090"), "адрес PromoGo API"),
		key:   fs.String("key", os.Getenv("PROMOGO_STORE_KEY"), "API-ключ магазина"),
		queue: fs.String("queue", envOr("PROMOGO_QUEUE", "possim-queue.jsonl"), "файл офлайн-очереди"),
	}
}

func (f commonFlags) env(needKey bool) (env, error) {
	if needKey && *f.key == "" {
		return env{}, errors.New("нужен API-ключ магазина: -key или PROMOGO_STORE_KEY")
	}
	return env{client: newClient(*f.url, *f.key), queue: queue{path: *f.queue}}, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// newTransactionID mimics a 1C document number: unique per operation and
// stable across every retry and replay of it.
func newTransactionID() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return "SIM-" + time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

func parseAmount(s string) (string, error) {
	d, err := decimal.NewFromString(strings.ReplaceAll(s, ",", "."))
	if err != nil || !d.IsPositive() {
		return "", fmt.Errorf("сумма %q: нужно положительное число, например 1250.00", s)
	}
	return d.StringFixed(2), nil
}

// drainFirst replays the offline queue before a new live operation, the
// way a register catches up once its link is back. A failure leaves the
// queue as it is; the new operation then queues behind it.
func drainFirst(ctx context.Context, e env) (online bool) {
	ops, err := e.queue.load()
	if err != nil || len(ops) == 0 {
		return err == nil
	}
	fmt.Printf("связь есть, операций в очереди: %d — досылаю…\n", len(ops))
	res, err := replay(ctx, e.client, e.queue, printReplay)
	printReplaySummary(res)
	return err == nil
}

func printReplay(op queuedOp, outcome string) {
	fmt.Printf("  %-6s %s: %s\n", op.Kind, op.transactionID(), outcome)
}

func printReplaySummary(r replayResult) {
	fmt.Printf("очередь: доставлено %d, уже было %d, отклонено %d, осталось %d\n",
		r.Delivered, r.Replayed, r.Rejected, r.Left)
}

// enqueue records an operation the register could not deliver.
func enqueue(e env, op queuedOp, cause error) error {
	op.QueuedAt = time.Now()
	if cause != nil {
		op.LastErr = cause.Error()
	}
	if err := e.queue.append(op); err != nil {
		return fmt.Errorf("запись в очередь: %w", err)
	}
	fmt.Printf("нет связи с API — %s %s поставлена в офлайн-очередь (%s)\n", op.Kind, op.transactionID(), e.queue.path)
	return nil
}

func runSale(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("sale", flag.ExitOnError)
	common := addCommon(fs)
	phone := fs.String("phone", "", "телефон клиента, например +79261234567")
	amount := fs.String("amount", "", "сумма покупки")
	id := fs.String("id", "", "transaction_id (по умолчанию новый); повтор с тем же id проверяет идемпотентность")
	offline := fs.Bool("offline", false, "не обращаться к API, сразу в очередь")
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	if *phone == "" {
		return errors.New("нужен -phone")
	}
	amt, err := parseAmount(*amount)
	if err != nil {
		return err
	}
	req := saleRequest{TransactionID: *id, Phone: *phone, Amount: amt}
	if req.TransactionID == "" {
		req.TransactionID = newTransactionID()
	}
	return checkoutSale(ctx, e, req, *offline)
}

func checkoutSale(ctx context.Context, e env, req saleRequest, offline bool) error {
	if offline || !drainFirst(ctx, e) {
		return enqueue(e, queuedOp{Kind: "sale", Sale: &req}, nil)
	}
	resp, err := e.client.sale(ctx, req)
	if isTransient(err) {
		return enqueue(e, queuedOp{Kind: "sale", Sale: &req}, err)
	}
	if err != nil {
		return err
	}
	fmt.Printf("продажа %s: %s ₽, %s, клиент %d, начислено %d, баланс %d%s\n",
		req.TransactionID, req.Amount, req.Phone, resp.ClientID, resp.PointsEarned, resp.Balance, replayedNote(resp.Replayed))
	return nil
}

func runRedeem(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("redeem", flag.ExitOnError)
	common := addCommon(fs)
	phone := fs.String("phone", "", "телефон клиента")
	points := fs.Int64("points", 0, "сколько баллов списать")
	amount := fs.String("amount", "", "сумма чека, от которой считается лимит списания")
	id := fs.String("id", "", "transaction_id (по умолчанию новый)")
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	if *phone == "" || *points <= 0 {
		return errors.New("нужны -phone и -points > 0")
	}
	amt, err := parseAmount(*amount)
	if err != nil {
		return err
	}

	// Paying with points needs a live balance, so there is no offline path.
	cl, found, err := e.client.lookup(ctx, *phone)
	if isTransient(err) {
		return fmt.Errorf("нет связи с API — оплата баллами недоступна офлайн: %w", err)
	}
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("клиент %s в этом магазине не найден", *phone)
	}

	req := redeemRequest{TransactionID: *id, ClientID: cl.ClientID, Points: *points, Amount: amt}
	if req.TransactionID == "" {
		req.TransactionID = newTransactionID()
	}
	resp, err := e.client.redeem(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("списание %s: клиент %d, списано %d, баланс %d%s\n",
		req.TransactionID, cl.ClientID, resp.PointsRedeemed, resp.Balance, replayedNote(resp.Replayed))
	return nil
}

func runRefund(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("refund", flag.ExitOnError)
	common := addCommon(fs)
	original := fs.String("original", "", "transaction_id исходной продажи или списания")
	typ := fs.String("type", "", "accrual или redeem — только если id использовался для обоих")
	amount := fs.String("amount", "", "сумма возврата (может быть частичной)")
	id := fs.String("id", "", "transaction_id возврата (по умолчанию <original>-R<случайное>)")
	offline := fs.Bool("offline", false, "не обращаться к API, сразу в очередь")
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	if *original == "" {
		return errors.New("нужен -original")
	}
	if *typ != "" && *typ != "accrual" && *typ != "redeem" {
		return errors.New("-type: accrual или redeem")
	}
	amt, err := parseAmount(*amount)
	if err != nil {
		return err
	}
	req := refundRequest{TransactionID: *id, OriginalTransactionID: *original, OriginalTransactionType: *typ, Amount: amt}
	if req.TransactionID == "" {
		var b [2]byte
		_, _ = rand.Read(b[:])
		req.TransactionID = *original + "-R" + hex.EncodeToString(b[:])
	}

	if *offline || !drainFirst(ctx, e) {
		return enqueue(e, queuedOp{Kind: "refund", Refund: &req}, nil)
	}
	resp, err := e.client.refund(ctx, req)
	if isTransient(err) {
		return enqueue(e, queuedOp{Kind: "refund", Refund: &req}, err)
	}
	if err != nil {
		return err
	}
	state := "частичный"
	if resp.FullyRefunded {
		state = "полный"
	}
	fmt.Printf("возврат %s по %s: %s ₽ (%s, всего возвращено %s), клиент %d, отменено баллов %d, баланс %d%s\n",
		req.TransactionID, *original, req.Amount, state, resp.RefundedAmountTotal, resp.ClientID,
		resp.PointsReversed, resp.Balance, replayedNote(resp.Replayed))
	return nil
}

func runBalance(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("balance", flag.ExitOnError)
	common := addCommon(fs)
	phone := fs.String("phone", "", "телефон клиента")
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	if *phone == "" {
		return errors.New("нужен -phone")
	}
	cl, found, err := e.client.lookup(ctx, *phone)
	if err != nil {
		return err
	}
	if !found {
		fmt.Printf("%s: в этом магазине ещё не покупал\n", *phone)
		return nil
	}
	fmt.Printf("%s: клиент %d, баланс %d\n", *phone, cl.ClientID, cl.Balance)
	return nil
}

func runQR(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("qr", flag.ExitOnError)
	common := addCommon(fs)
	payload := fs.String("payload", "", "содержимое QR из мобильного приложения (POST /me/qr)")
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	if *payload == "" {
		return errors.New("нужен -payload")
	}
	cl, err := e.client.resolveQR(ctx, *payload)
	if err != nil {
		return err
	}
	fmt.Printf("QR опознан: клиент %d, баланс %d\n", cl.ClientID, cl.Balance)
	return nil
}

func runReplay(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	common := addCommon(fs)
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	ops, err := e.queue.load()
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		fmt.Println("очередь пуста")
		return nil
	}
	res, err := replay(ctx, e.client, e.queue, printReplay)
	printReplaySummary(res)
	if err != nil {
		return fmt.Errorf("связи всё ещё нет, досылка остановлена: %w", err)
	}
	return nil
}

func runQueue(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("queue", flag.ExitOnError)
	common := addCommon(fs)
	fs.Parse(args)

	e, err := common.env(false)
	if err != nil {
		return err
	}
	ops, err := e.queue.load()
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		fmt.Println("очередь пуста")
		return nil
	}
	for _, op := range ops {
		line := fmt.Sprintf("%s  %-6s %s", op.QueuedAt.Local().Format("02.01 15:04:05"), op.Kind, op.transactionID())
		switch {
		case op.Sale != nil:
			line += fmt.Sprintf("  %s ₽  %s", op.Sale.Amount, op.Sale.Phone)
		case op.Refund != nil:
			line += fmt.Sprintf("  %s ₽  по %s", op.Refund.Amount, op.Refund.OriginalTransactionID)
		}
		if op.LastErr != "" {
			line += "  [" + op.LastErr + "]"
		}
		fmt.Println(line)
	}
	fmt.Printf("всего: %d\n", len(ops))
	return nil
}

func replayedNote(replayed bool) string {
	if replayed {
		return " — повтор: операция уже была принята и второй раз не проведена"
	}
	return ""
}
