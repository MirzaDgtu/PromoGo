// Package metrics holds this service's Prometheus collectors. They are
// process-wide singletons (the idiomatic Prometheus client pattern —
// scraped state, not a per-request dependency), registered against the
// default registry via promauto, and served at GET /metrics
// (internal/httpserver/server.go).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestDuration is labeled by operation_id (routeMeta.OperationID —
// stable and low-cardinality, unlike the raw URL path) rather than the path
// itself, method, and status class, so it never accumulates a label series
// per numeric ID in a URL.
var HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "promogo_http_request_duration_seconds",
	Help:    "HTTP request latency in seconds, by route and outcome.",
	Buckets: prometheus.DefBuckets,
}, []string{"operation_id", "method", "status"})

// IdempotencyConflicts counts a replayed webhook whose request fingerprint
// doesn't match the original (domain.ErrIdempotencyConflict) — a real
// integrity signal: 1C or a caller retried with different data under the
// same external_tx_id.
var IdempotencyConflicts = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "promogo_idempotency_conflicts_total",
	Help: "Idempotency-key replays whose request fingerprint didn't match the original write.",
}, []string{"flow"}) // flow: accrue|redeem|refund

// RefundFailures counts a refund request that failed business validation
// (unknown original transaction, over-refund, etc.), labeled by the
// specific reason so a spike in one reason is distinguishable from another.
var RefundFailures = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "promogo_refund_failures_total",
	Help: "Refund requests rejected by business validation, by reason.",
}, []string{"reason"})

// OTPIssued and OTPVerified cover the customer OTP login flow's two steps.
var (
	OTPIssued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promogo_otp_issued_total",
		Help: "OTP issue attempts, by outcome.",
	}, []string{"result"}) // result: success|rate_limited|error

	OTPVerified = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promogo_otp_verified_total",
		Help: "OTP verification attempts, by outcome.",
	}, []string{"result"}) // result: success|invalid|expired|error
)

// QRConsume covers QR resolve outcomes at POS: whether the token was
// consumed and finalized, hit a definitive business failure (also
// finalized — see internal/service/qr.go's ResolveQR), or a transient
// infrastructure failure (released for retry).
var QRConsume = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "promogo_qr_consume_total",
	Help: "QR resolve/consume attempts, by outcome.",
}, []string{"result"}) // result: success|business_failure|transient_failure

// RateLimitRejections counts a request rejected with 429, by rate-limit
// profile (see internal/ratelimit).
var RateLimitRejections = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "promogo_rate_limit_rejections_total",
	Help: "Requests rejected with 429, by rate-limit profile.",
}, []string{"profile"})

// OutboxDeliveryOutcomes and OutboxBacklog cover the notification outbox
// (internal/service/outbox_worker.go): every delivery attempt's resolution,
// and how many entries are currently pending/processing (sampled each poll)
// — a growing backlog means delivery is falling behind enqueue rate.
var (
	OutboxDeliveryOutcomes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "promogo_outbox_delivery_outcomes_total",
		Help: "Notification outbox delivery attempts, by outcome.",
	}, []string{"outcome"}) // outcome: delivered|retried|dead_letter

	OutboxBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promogo_outbox_backlog",
		Help: "Notification outbox rows currently pending or processing, sampled each poll.",
	})
)
