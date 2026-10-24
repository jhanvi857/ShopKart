package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func ObservabilityMiddleware(serviceName string, m *metrics.Metrics) func(http.Handler) http.Handler {
	tracer := otel.GetTracerProvider().Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics/tracing for internal health and metrics scraping endpoints to avoid noise
			if r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Extract incoming trace context
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethodKey.String(r.Method),
					semconv.HTTPTargetKey.String(r.URL.Path),
					semconv.NetHostNameKey.String(r.Host),
				),
			)
			defer span.End()

			m.RequestsInFlight.WithLabelValues().Inc()
			defer m.RequestsInFlight.WithLabelValues().Dec()

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			r = r.WithContext(ctx)

			defer func() {
				if rec := recover(); rec != nil {
					rw.statusCode = http.StatusInternalServerError
					span.RecordError(nil)
					slog.ErrorContext(ctx, "panic recovered in http handler",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
					)
					http.Error(rw, `{"error":"internal server error"}`, http.StatusInternalServerError)
				}

				duration := time.Since(start)

				// Determine route pattern if chi router matched
				routePattern := r.URL.Path
				if rCtx := chi.RouteContext(r.Context()); rCtx != nil {
					pattern := rCtx.RoutePattern()
					if pattern != "" {
						routePattern = pattern
					}
				}

				span.SetAttributes(
					semconv.HTTPStatusCodeKey.Int(rw.statusCode),
					attribute.String("http.route", routePattern),
				)

				m.TrackRequest(r.Method, routePattern, rw.statusCode, duration)

				slog.InfoContext(ctx, "handled request",
					slog.String("method", r.Method),
					slog.String("path", routePattern),
					slog.Int("status", rw.statusCode),
					slog.Duration("latency", duration),
					slog.String("remote_addr", r.RemoteAddr),
				)
			}()

			next.ServeHTTP(rw, r)
		})
	}
}
