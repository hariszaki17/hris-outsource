// Package log configures the process-wide structured logger (slog). JSON in
// production, text in dev. The request_id is added per-line by handlers/services
// from context (httpx.RequestID); traces correlate via the obs package.
package log

import (
	"log/slog"
	"os"
	"strings"
)

// Setup installs a slog default logger and returns it.
func Setup(env, level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var h slog.Handler
	if env == "production" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
