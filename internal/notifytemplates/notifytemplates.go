// Package notifytemplates centralizes the accrual/redeem/refund
// notification text that used to be hardcoded fmt.Sprintf calls inside
// internal/service/loyalty.go (Phase 3 of docs/audit-remediation-prompt.md:
// "Move notification text into versioned/localizable templates ...
// instead of hardcoding Russian strings in loyalty services").
//
// Each template has a stable ID and an integer Version, both persisted on
// the outbox row that renders from it (see domain.NotificationOutboxEntry)
// — so a later wording change doesn't retroactively alter what a historical
// notification is understood to have said, and a future delivery-metrics
// or support tool can group/filter by template independent of the
// rendered text.
//
// Localization and deep links are NOT implemented here: the MVP ships
// Russian-only text with no in-app navigation target, matching current
// behavior exactly. Q-P0-062 (knowledge/Project Questions.md) tracks the
// open product decision on languages, deep-link targets, and template
// ownership/approval — this package is the seam that decision plugs into
// (a Render function keyed by locale, and a DeepLink field alongside
// Message) without being a guess at product scope this repo doesn't own.
package notifytemplates

import "fmt"

// Rendered is one notification's rendered content plus the template
// identity that produced it.
type Rendered struct {
	TemplateID string
	Version    int
	Message    string
}

const (
	accrualCreditedID = "accrual_credited"
	redeemDebitedID   = "redeem_debited"
	refundAdjustedID  = "refund_adjusted"
	// templateVersion applies to all three templates in this package —
	// bump it (and keep the old rendering available if in-flight outbox
	// rows must remain interpretable) when wording changes materially.
	templateVersion = 1
)

// AccrualCredited renders the notification sent when a purchase earns
// points.
func AccrualCredited(points int64) Rendered {
	return Rendered{
		TemplateID: accrualCreditedID,
		Version:    templateVersion,
		Message:    fmt.Sprintf("Вам начислено %d баллов", points),
	}
}

// RedeemDebited renders the notification sent when points are redeemed.
func RedeemDebited(points int64) Rendered {
	return Rendered{
		TemplateID: redeemDebitedID,
		Version:    templateVersion,
		Message:    fmt.Sprintf("Списано %d баллов", points),
	}
}

// RefundAdjusted renders the notification sent when a refund adjusts a
// client's points balance. pointsDelta carries its own sign (e.g. -20 for
// a reversed accrual), matching RefundResult.PointsReversed.
func RefundAdjusted(pointsDelta int64) Rendered {
	return Rendered{
		TemplateID: refundAdjustedID,
		Version:    templateVersion,
		Message:    fmt.Sprintf("Возврат: скорректировано %d баллов", pointsDelta),
	}
}
