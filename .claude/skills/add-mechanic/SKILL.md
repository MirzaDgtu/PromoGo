---
name: add-mechanic
description: Scaffold a new domain.Mechanic implementation for PromoGo under internal/mechanic/<name> and wire it into internal/mechanicbuild. Use when asked to add, implement, or plug in a new loyalty mechanic (cashback, punch card, tiers, etc.).
---

# Adding a loyalty mechanic to PromoGo

MVP ships one mechanic, "points" (flat % of purchase amount). The remaining mechanics from
`Idea.md` (cashback, punch card, tiers, category bonuses) are explicitly out of MVP scope
(see `MVP-scope.md`'s cut list) but the architecture is built to add them without touching
`LoyaltyService`.

A mechanic implements `domain.Mechanic` (`internal/domain/mechanic.go`):

```go
type Mechanic interface {
    Name() string
    Accrue(ctx context.Context, tx *Transaction, cfg *LoyaltyConfig, balance *Balance) (pointsEarned int64, err error)
}
```

`Accrue` runs once per accrual transaction (`LoyaltyService.Accrue`, in `internal/service/loyalty.go`).
Return `0, nil` for "doesn't qualify" (e.g. below a minimum purchase amount) — that's not an
error. A positive return credits that many points, applied atomically alongside the transaction
record by `LedgerRepository.Post` (`internal/repository/postgres/ledger_repository.go`) — the
mechanic itself never touches the database.

## Steps

1. **Create the package** `internal/mechanic/<name>/<name>.go`. Use
   `internal/mechanic/points/points.go` as the template — same shape: a constructor `New(...)`,
   a `Name() string`, and `Accrue`.
2. **Use `decimal.Decimal`** for any money math (`github.com/shopspring/decimal`) — `tx.Amount`
   and every `LoyaltyConfig` percentage field are `decimal.Decimal`, never `float64`. Points
   themselves are `int64` (a count, not money) — round with `.Floor().IntPart()` when converting
   a decimal computation to a point count, matching `points.go`.
3. **Read whatever `LoyaltyConfig` fields you need.** If the mechanic needs a parameter that
   doesn't exist yet (e.g. punch-card's "purchases needed" count, tiers' per-tier rate table),
   add a field to `domain.LoyaltyConfig` (`internal/domain/loyalty_config.go`), then to the
   `loyalty_configs` table via the db-migrate skill, then to
   `postgres.LoyaltyConfigRepository`'s query/scan (`internal/repository/postgres/loyalty_config_repository.go`).
   Keep new fields nullable/defaulted so the existing "points" mechanic config rows stay valid.
4. **Use `balance`** if the mechanic needs the client's current state (tiers need the current
   tier, punch card needs the running stamp count — which would itself need new state beyond
   `Balance.Points`, e.g. a `PunchCount` field or a separate table; don't force everything into
   `Balance.Points`).
5. **Write a table-driven test** `<name>_test.go` alongside it, covering: below-minimum (no
   points), at-minimum, rounding behavior, and any config edge case — mirror `points_test.go`.
6. **Wire it into `internal/mechanicbuild/mechanicbuild.go`**: add a case to the switch in
   `Build(name string)`.
7. **Users select it** by setting `loyalty_configs.mechanic` to your mechanic's `Name()` — there's
   no HTTP endpoint to configure this yet (MVP's web configurator is out of scope, see
   `MVP-scope.md`); it's set directly via SQL for now (see the dev-stack skill's seed example).
8. **Build and test**: `go build ./...` and `go test ./internal/mechanic/...`.

## Notes

- A mechanic is pure decision logic — no side effects (persistence, notifications). Persisting
  the transaction and adjusting the balance is `LedgerRepository.Post`'s job, and it does both
  atomically in one database transaction; see `internal/domain/ledger.go`'s doc comment for why
  that atomicity matters (a mechanic returning a value is not itself risky, but don't be tempted
  to add a repository call inside `Accrue` — that would bypass the atomic post).
- Only one mechanic runs per store per transaction (`LoyaltyConfig.Mechanic` selects it) — unlike
  CoinGoBot's strategies, which can stack multiple per symbol. If a future requirement needs
  multiple mechanics stacking (e.g. points + a running promotion multiplier), that's a
  `LoyaltyService.Accrue` change, not a `Mechanic` interface change.
