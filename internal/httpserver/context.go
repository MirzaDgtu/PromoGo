package httpserver

import (
	"context"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

type storeContextKey struct{}

// storeFromContext returns the domain.Store resolved by RequireStoreAPIKey
// for the current request.
func storeFromContext(ctx context.Context) (*domain.Store, bool) {
	store, ok := ctx.Value(storeContextKey{}).(*domain.Store)
	return store, ok
}

type customerContextKey struct{}

// customerFromContext returns the CustomerAccountID resolved by
// RequireCustomerSession for the current request.
func customerFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(customerContextKey{}).(int64)
	return id, ok
}

type staffContextKey struct{}

// staffFromContext returns the *service.StaffPrincipal resolved by
// RequireStaff for the current request.
func staffFromContext(ctx context.Context) (*service.StaffPrincipal, bool) {
	principal, ok := ctx.Value(staffContextKey{}).(*service.StaffPrincipal)
	return principal, ok
}
