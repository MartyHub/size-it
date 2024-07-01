package internal

import (
	"log/slog"
	"os"
)

const (
	LogKeyError   = "error"
	LogKeyLatency = "latency"
	LogKeyMethod  = "method"
	LogKeyURI     = "uri"
	LogKeyStatus  = "status"
	LogKeyUser    = "user"
)

func ConfigureLogs(dev bool) {
	var hdl slog.Handler

	out := os.Stdout
	opts := &slog.HandlerOptions{}

	if dev {
		opts.Level = slog.LevelDebug
		hdl = slog.NewTextHandler(out, opts)
	} else {
		hdl = slog.NewJSONHandler(out, opts)
	}

	slog.SetDefault(slog.New(hdl))
}

func LogError(msg string, err error) {
	slog.Error(msg, slog.String(LogKeyError, err.Error()))
}
