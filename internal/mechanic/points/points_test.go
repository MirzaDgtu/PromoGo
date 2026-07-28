package points

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func TestMechanic_Accrue(t *testing.T) {
	cfg := &domain.LoyaltyConfig{
		AccrualPercent:    decimal.NewFromInt(5),
		MinPurchaseAmount: decimal.NewFromInt(100),
	}

	cases := []struct {
		name   string
		amount decimal.Decimal
		want   int64
	}{
		{"below minimum purchase", decimal.NewFromInt(50), 0},
		{"at minimum purchase", decimal.NewFromInt(100), 5},
		{"above minimum, rounds down", decimal.NewFromFloat(199.90), 9},
		{"large purchase", decimal.NewFromInt(1500), 75},
	}

	m := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &domain.Transaction{Amount: tc.amount}
			got, err := m.Accrue(context.Background(), tx, cfg, &domain.Balance{})
			if err != nil {
				t.Fatalf("Accrue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("Accrue() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMechanic_Name(t *testing.T) {
	if got := New().Name(); got != "points" {
		t.Errorf("Name() = %q, want %q", got, "points")
	}
}
