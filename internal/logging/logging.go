package logging

import (
	"context"
	"log/slog"
	"os"
)

type LevelHandler struct {
	level slog.Level
	slog.Handler
}

func NewLevelHandler(level slog.Level, opts *slog.HandlerOptions) *LevelHandler {
	return &LevelHandler{
		level:   level,
		Handler: slog.NewTextHandler(os.Stdout),
	}
}

//func (h *LevelHandler)

type Handler interface {
	Enabled(context.Context, Level) bool
	Handle(context.Context, Record) error
	WithAttrs(attrs []Attr) Handler
	WithGroup(name string) Handler
}
