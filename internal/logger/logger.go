package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/MirzaDgtu/PromoGo/internal/config"
)

// New builds a slog.Logger configured according to cfg. Format may be
// "json" (default) or "text"; Level may be debug, info, warn, or error.
func New(cfg config.LoggerConfig) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
