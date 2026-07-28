// Package mechanicbuild constructs domain.Mechanic implementations by name,
// for a store's configured LoyaltyConfig.Mechanic.
package mechanicbuild

import (
	"fmt"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/mechanic/points"
)

// Build returns the domain.Mechanic registered under name, or an error if
// name doesn't match any known mechanic. Add a case here (and a
// corresponding package under internal/mechanic) when adding a new mechanic.
func Build(name string) (domain.Mechanic, error) {
	switch name {
	case "points":
		return points.New(), nil
	default:
		return nil, fmt.Errorf("unknown mechanic %q", name)
	}
}
