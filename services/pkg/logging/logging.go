package logging

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type TraceHandler struct {
	slog.Handler
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}

func InitLogger(serviceName, instanceID, env string) *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	traceHandler := &TraceHandler{Handler: jsonHandler}
	logger := slog.New(traceHandler).With(
		slog.String("service", serviceName),
		slog.String("instance", instanceID),
		slog.String("env", env),
	)

	slog.SetDefault(logger)
	return logger
}
