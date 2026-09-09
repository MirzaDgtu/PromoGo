package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/repository/postgres"
)

// loadtestSeedOutput is what runLoadtestSeed prints on stdout as JSON —
// docs/loadtest/*.js scripts read this to get a working store API key
// without hand-editing SQL against whatever the current schema looks like
// (see .claude/skills/dev-stack/SKILL.md's raw-SQL seed, which this avoids
// duplicating and which drifts every time the stores/loyalty_configs shape
// changes).
type loadtestSeedOutput struct {
	OrganizationID int64  `json:"organization_id"`
	StoreID        int64  `json:"store_id"`
	APIKey         string `json:"api_key"`
}

// runLoadtestSeed creates a throwaway organization, store, fully-scoped
// store API key, and loyalty config directly against Postgres — the same
// bypass-HTTP pattern as bootstrap-admin, used here so load/soak tests
// (loadtest/*.js) don't need a real OIDC-authenticated staff session just
// to get a working store credential. Never run this against a database
// that isn't a disposable load-test target.
func runLoadtestSeed(args []string) {
	fs := flag.NewFlagSet("loadtest-seed", flag.ExitOnError)
	name := fs.String("name", "Loadtest Store", "name recorded for the seeded store")
	fs.Parse(args)

	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN(), cfg.Postgres.MaxConns)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	orgs := postgres.NewOrganizationRepository(pool)
	stores := postgres.NewStoreRepository(pool)
	apiKeys := postgres.NewStoreAPIKeyRepository(pool)
	configs := postgres.NewLoyaltyConfigRepository(pool)

	org := &domain.Organization{Name: *name + " Org"}
	if err := orgs.Create(ctx, org); err != nil {
		log.Fatalf("create organization: %v", err)
	}

	store := &domain.Store{OrganizationID: org.ID, Name: *name}
	if err := stores.Create(ctx, store); err != nil {
		log.Fatalf("create store: %v", err)
	}

	plaintext, keyID, hash, err := auth.GenerateAPIKey()
	if err != nil {
		log.Fatalf("generate api key: %v", err)
	}
	if err := apiKeys.Create(ctx, &domain.StoreAPIKey{
		StoreID: store.ID,
		KeyID:   keyID,
		KeyHash: hash,
		Name:    "loadtest",
		Scopes:  []string{domain.ScopeTransactionsWrite, domain.ScopeClientsLookup, domain.ScopeBalancesRead},
	}); err != nil {
		log.Fatalf("create store api key: %v", err)
	}

	cfg2 := &domain.LoyaltyConfig{
		StoreID:            store.ID,
		Mechanic:           "points",
		AccrualPercent:     decimal.NewFromInt(5),
		MinPurchaseAmount:  decimal.Zero,
		MinBalanceToRedeem: 0,
		MaxRedeemPercent:   decimal.NewFromInt(50),
		PointsExchangeRate: decimal.NewFromInt(1),
	}
	if err := configs.Upsert(ctx, cfg2, nil); err != nil {
		log.Fatalf("create loyalty config: %v", err)
	}

	out := loadtestSeedOutput{OrganizationID: org.ID, StoreID: store.ID, APIKey: plaintext}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		log.Fatalf("encode output: %v", err)
	}
	fmt.Fprintf(os.Stderr, "seeded store %d (org %d) — api key printed to stdout as JSON\n", store.ID, org.ID)
}
