// Package log configures the default slog logger with fractional-second timestamps.
// Import this package for its side effects.
package log

import (
	"log/slog"
	"os"
)

func init() {
	// time.RFC3339Nano but with shorter and fixed width fractional seconds
	const timeFormat = "2006-01-02T15:04:05.000Z07:00"

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(timeFormat))
			}
			return a
		},
	})))
}
