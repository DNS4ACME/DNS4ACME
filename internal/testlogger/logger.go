package testlogger

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func New(t *testing.T) *slog.Logger {
	t.Helper()
	handler := slog.NewTextHandler(NewWriter(t), &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return slog.New(handler)
}

func NewWriter(t *testing.T) io.Writer {
	return &testHandler{t}
}

type testHandler struct {
	t *testing.T
}

func (t testHandler) Write(p []byte) (n int, err error) {
	t.t.Helper()
	lines := strings.Split(strings.TrimSpace(string(p)), "\n")
	for _, line := range lines {
		t.t.Log(line)
	}
	return len(p), nil
}
