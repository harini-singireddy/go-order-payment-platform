package logger

import (
	"log/slog"
	"os"
)

// New returns a structured JSON logger tagged with the given service name.
// Every log line includes "service" so logs can be filtered in Grafana/Loki.
func New(service string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler).With("service", service)
}
