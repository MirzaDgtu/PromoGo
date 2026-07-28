package httpserver

import (
	"context"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type storeContextKey struct{}

// storeFromContext returns the domain.Store resolved by RequireStoreAPIKey
// for the current request.
func storeFromContext(ctx context.Context) (*domain.Store, bool) {
	store, ok := ctx.Value(storeContextKey{}).(*domain.Store)
	return store, ok
}
