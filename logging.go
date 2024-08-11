package htmplx

import (
	"log/slog"
	"os"
)

var env_htmplx_loglevel = os.Getenv("HTMPLX_LOGLEVEL")

func newLogger() *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(env_htmplx_loglevel)); err != nil {
		lvl = slog.LevelInfo
	}

	return slog.New(
		slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				AddSource: true,
				Level:     lvl,
			},
		),
	)
}
