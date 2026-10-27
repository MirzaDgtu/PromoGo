package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// runFlood rings up a stream of checkouts from a fixed pool of customers,
// so the Admin Web's overview, clients and audit screens fill with
// plausible data. Each checkout follows the register flow: identify the
// customer by phone, sometimes let them pay part of the bill with points,
// then send the sale.
func runFlood(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("flood", flag.ExitOnError)
	common := addCommon(fs)
	n := fs.Int("n", 20, "сколько продаж провести")
	clients := fs.Int("clients", 8, "сколько разных покупателей (+79990000001, +79990000002, …)")
	redeemShare := fs.Float64("redeem-share", 0.3, "доля покупок, где клиент с баллами платит ими часть чека")
	minAmount := fs.Float64("min", 150, "минимальная сумма чека")
	maxAmount := fs.Float64("max", 4500, "максимальная сумма чека")
	pause := fs.Duration("pause", 0, "пауза между продажами, например 200ms")
	seed := fs.Uint64("seed", 0, "зерно случайности (0 — каждый раз новое)")
	offline := fs.Bool("offline", false, "не обращаться к API, все продажи сразу в очередь")
	fs.Parse(args)

	e, err := common.env(true)
	if err != nil {
		return err
	}
	if *n <= 0 || *clients <= 0 || *minAmount <= 0 || *maxAmount < *minAmount {
		return errors.New("нужны -n > 0, -clients > 0 и 0 < -min ≤ -max")
	}
	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}
	rng := rand.New(rand.NewPCG(*seed, *seed>>1))

	var stats floodStats
	for i := range *n {
		if ctx.Err() != nil {
			break
		}
		phone := fmt.Sprintf("+7999%07d", 1+rng.IntN(*clients))
		amount := decimal.NewFromFloat(*minAmount + rng.Float64()*(*maxAmount-*minAmount)).Round(2)
		id := newTransactionID() + fmt.Sprintf("-%03d", i+1)

		if !*offline && rng.Float64() < *redeemShare {
			payWithPoints(ctx, e, id, phone, amount, &stats)
		}
		if err := floodSale(ctx, e, saleRequest{TransactionID: id, Phone: phone, Amount: amount.StringFixed(2)}, *offline, &stats); err != nil {
			return err
		}
		if *pause > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(*pause):
			}
		}
	}

	fmt.Printf("\nитого: продаж %d на %s ₽, начислено %d; списаний %d на %d баллов (отклонено %d); в очередь %d; отклонено продаж %d\n",
		stats.sales, stats.salesAmount.StringFixed(2), stats.accrued,
		stats.redeems, stats.redeemed, stats.redeemRejected, stats.queued, stats.rejected)
	fmt.Printf("зерно: %d (повторить тот же поток: -seed %d)\n", *seed, *seed)
	return nil
}

type floodStats struct {
	sales, rejected, queued int
	salesAmount             decimal.Decimal
	accrued                 int64
	redeems, redeemRejected int
	redeemed                int64
}

// payWithPoints lets a returning customer cover up to a tenth of the bill
// with points. The store's own rules (min balance, max redeem percent)
// may refuse it; then the customer simply pays in full, as at a real till.
func payWithPoints(ctx context.Context, e env, id, phone string, amount decimal.Decimal, s *floodStats) {
	cl, found, err := e.client.lookup(ctx, phone)
	if err != nil || !found || cl.Balance <= 0 {
		return
	}
	points := min(cl.Balance, max(1, amount.Div(decimal.NewFromInt(10)).IntPart()))
	resp, err := e.client.redeem(ctx, redeemRequest{TransactionID: id, ClientID: cl.ClientID, Points: points, Amount: amount.StringFixed(2)})
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
			s.redeemRejected++
			fmt.Printf("списание %s: %s хотел списать %d — отказ правил магазина (%s)\n", id, phone, points, apiErr.Message)
			return
		}
		fmt.Printf("списание %s: %v\n", id, err)
		return
	}
	s.redeems++
	s.redeemed += resp.PointsRedeemed
	fmt.Printf("списание %s: %s −%d баллов, баланс %d\n", id, phone, resp.PointsRedeemed, resp.Balance)
}

func floodSale(ctx context.Context, e env, req saleRequest, offline bool, s *floodStats) error {
	// Once anything is queued, later sales must not overtake it: catch the
	// queue up first, and queue behind it while the link is still down.
	if offline || !drainFirst(ctx, e) {
		s.queued++
		return enqueue(e, queuedOp{Kind: "sale", Sale: &req}, nil)
	}
	resp, err := e.client.sale(ctx, req)
	switch {
	case isTransient(err):
		s.queued++
		return enqueue(e, queuedOp{Kind: "sale", Sale: &req}, err)
	case err != nil:
		s.rejected++
		fmt.Printf("продажа %s: отклонена: %v\n", req.TransactionID, err)
		return nil
	}
	s.sales++
	s.salesAmount = s.salesAmount.Add(decimal.RequireFromString(req.Amount))
	s.accrued += resp.PointsEarned
	fmt.Printf("продажа %s: %s ₽, %s, +%d, баланс %d\n", req.TransactionID, req.Amount, req.Phone, resp.PointsEarned, resp.Balance)
	return nil
}
