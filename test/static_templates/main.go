package main

import (
	_ "embed"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/angelbeltran/htmplx"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	type Data struct {
		URL    *url.URL
		Field1 string
		Field2 int
		htmplx.PathExpressionSubmatches
	}

	h := htmplx.NewHandler[Data](os.DirFS("public")).
		WithData(func(r *http.Request) Data {
			return Data{
				URL:                      r.URL,
				Field1:                   "test data",
				Field2:                   123,
				PathExpressionSubmatches: htmplx.PathExpressionSubmatches{},
			}
		})

	srv := http.Server{
		Addr:    ":8081",
		Handler: h,
	}

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		slog.With("error", err).
			Error("server shutdown with unexpected error")
	}
}
