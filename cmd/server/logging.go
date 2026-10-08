package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/teslacost/teslacost/internal/config"
)

// requestIDHandler wraps a slog.Handler to attach the chi request ID (if any is present on the
// context) to every log record. This is what lets a "request_id" field emitted by a *Context
// slog call (e.g. slog.ErrorContext in writeRepoError) be correlated with the chi access log
// line for the same request.
type requestIDHandler struct {
	slog.Handler
}

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID := chiMiddleware.GetReqID(ctx); reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{h.Handler.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{h.Handler.WithGroup(name)}
}

// configureLogging sets the process-wide slog default: JSON output in production (log
// aggregators, jq-friendly), human-readable text otherwise. Level defaults to Info in
// production (never Debug) and Debug in development; LOG_LEVEL overrides either.
func configureLogging(cfg *config.Config) {
	level := slog.LevelInfo
	if !strings.EqualFold(cfg.Environment, "production") {
		level = slog.LevelDebug
	}
	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		var parsed slog.Level
		if err := parsed.UnmarshalText([]byte(strings.ToUpper(raw))); err == nil {
			level = parsed
		}
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.Environment, "production") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(requestIDHandler{handler}))
}
