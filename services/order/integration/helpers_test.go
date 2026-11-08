package integration

import (
	"io"
	"log/slog"
	"testing"
)

func testLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
