package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type GatewayApp struct {
	tracer  trace.Tracer
	metrics *metrics.Metrics
}

func createProxy(targetURL string, pathPrefix string, tracer trace.Tracer) http.Handler {
	u, err := url.Parse(targetURL)
	if err != nil {
		panic(err)
	}

	// Custom transport with short idle connection timeout so Docker DNS changes & scaling are picked up
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = transport

	originalDirector := proxy.Director
	var rrCounter uint64

	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = u.Host

		// Strip prefix if necessary
		req.URL.Path = strings.TrimPrefix(req.URL.Path, pathPrefix)
		if !strings.HasPrefix(req.URL.Path, "/") {
			req.URL.Path = "/" + req.URL.Path
		}

		// Inject trace headers downstream
		tracing.InjectHTTPHeaders(req.Context(), req)
		atomic.AddUint64(&rrCounter, 1)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.ErrorContext(r.Context(), "gateway proxy error",
			slog.String("target", targetURL),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		http.Error(w, fmt.Sprintf(`{"error":"upstream service unavailable: %s"}`, err.Error()), http.StatusBadGateway)
	}

	return proxy
}

func main() {
	cfg := config.LoadCommon("edge-gateway", "9000")
	logger := logging.InitLogger(cfg.ServiceName, cfg.InstanceID, cfg.Environment)
	m := metrics.New(cfg.ServiceName, cfg.InstanceID, cfg.Environment)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := tracing.InitTracer(ctx, cfg.ServiceName, cfg.InstanceID, cfg.Environment, cfg.OTLPEndpoint)
	if err != nil {
		logger.Warn("Failed to initialize OTel tracer", slog.String("error", err.Error()))
	} else {
		defer func() { _ = tp.Shutdown(ctx) }()
	}
	tracer := otel.GetTracerProvider().Tracer(cfg.ServiceName)

	catalogURL := config.GetEnv("CATALOG_URL", "http://localhost:9001")
	searchURL := config.GetEnv("SEARCH_URL", "http://localhost:9002")
	checkoutURL := config.GetEnv("CHECKOUT_URL", "http://localhost:9003")
	inventoryURL := config.GetEnv("INVENTORY_URL", "http://localhost:9004")
	paymentsURL := config.GetEnv("PAYMENTS_URL", "http://localhost:9005")
	cartURL := config.GetEnv("CART_URL", "http://localhost:9006")
	reviewsURL := config.GetEnv("REVIEWS_URL", "http://localhost:9009")

	checker := health.NewChecker()

	r := chi.NewRouter()

	// CORS config
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Admin-Key", "traceparent"},
		ExposedHeaders:   []string{"Link", "X-Cache", "Content-Length"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(shopkartmw.ObservabilityMiddleware(cfg.ServiceName, m))

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())
	r.Handle("/metrics", m.Handler())

	// Upstream routes
	r.Mount("/api/catalog", createProxy(catalogURL, "/api/catalog", tracer))
	r.Mount("/api/search", createProxy(searchURL, "/api/search", tracer))
	r.Mount("/api/cart", createProxy(cartURL, "/api/cart", tracer))
	r.Mount("/api/checkout", createProxy(checkoutURL, "/api/checkout", tracer))
	r.Mount("/api/orders", createProxy(checkoutURL, "/api/orders", tracer))
	r.Mount("/api/inventory", createProxy(inventoryURL, "/api/inventory", tracer))
	r.Mount("/api/payments", createProxy(paymentsURL, "/api/payments", tracer))
	r.Mount("/api/reviews", createProxy(reviewsURL, "/api/reviews", tracer))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		logger.Info("Starting edge-gateway", slog.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	logger.Info("Edge-gateway shutdown gracefully")
}
