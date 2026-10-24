package main

import (
	"context"
	"encoding/json"
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

	pools := map[string]*ServicePool{
		"catalog":   NewServicePool(ctx, "catalog", "/api/catalog", parseUpstreamAddrs(config.GetEnv("CATALOG_UPSTREAMS", ""), config.GetEnv("CATALOG_URL", "http://localhost:9001")), tracer),
		"search":    NewServicePool(ctx, "search", "/api/search", parseUpstreamAddrs(config.GetEnv("SEARCH_UPSTREAMS", ""), config.GetEnv("SEARCH_URL", "http://localhost:9002")), tracer),
		"cart":      NewServicePool(ctx, "cart", "/api/cart", parseUpstreamAddrs(config.GetEnv("CART_UPSTREAMS", ""), config.GetEnv("CART_URL", "http://localhost:9006")), tracer),
		"checkout":  NewServicePool(ctx, "checkout", "/api/checkout", parseUpstreamAddrs(config.GetEnv("CHECKOUT_UPSTREAMS", ""), config.GetEnv("CHECKOUT_URL", "http://localhost:9003")), tracer),
		"orders":    NewServicePool(ctx, "orders", "/api/orders", parseUpstreamAddrs(config.GetEnv("CHECKOUT_UPSTREAMS", ""), config.GetEnv("CHECKOUT_URL", "http://localhost:9003")), tracer),
		"inventory": NewServicePool(ctx, "inventory", "/api/inventory", parseUpstreamAddrs(config.GetEnv("INVENTORY_UPSTREAMS", ""), config.GetEnv("INVENTORY_URL", "http://localhost:9004")), tracer),
		"payments":  NewServicePool(ctx, "payments", "/api/payments", parseUpstreamAddrs(config.GetEnv("PAYMENTS_UPSTREAMS", ""), config.GetEnv("PAYMENTS_URL", "http://localhost:9005")), tracer),
		"reviews":   NewServicePool(ctx, "reviews", "/api/reviews", parseUpstreamAddrs(config.GetEnv("REVIEWS_UPSTREAMS", ""), config.GetEnv("REVIEWS_URL", "http://localhost:9009")), tracer),
	}

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

	// Admin endpoints for dynamic upstream management
	r.Get("/admin/upstreams", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Admin-Key") != cfg.AdminAPIKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		status := make(map[string][]UpstreamInfo)
		for svc, pool := range pools {
			status[svc] = pool.GetStatus()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	})

	r.Post("/admin/upstreams", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Admin-Key") != cfg.AdminAPIKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		var payload struct {
			Service  string `json:"service"`
			Upstream string `json:"upstream"`
			Action   string `json:"action"` // "add" or "remove"
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
			return
		}
		pool, exists := pools[payload.Service]
		if !exists {
			http.Error(w, fmt.Sprintf(`{"error":"unknown service %s"}`, payload.Service), http.StatusNotFound)
			return
		}
		if payload.Action == "remove" {
			pool.RemoveUpstream(payload.Upstream)
			if payload.Service == "checkout" {
				pools["orders"].RemoveUpstream(payload.Upstream)
			}
		} else {
			pool.AddUpstream(payload.Upstream)
			if payload.Service == "checkout" {
				pools["orders"].AddUpstream(payload.Upstream)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"service":  payload.Service,
			"upstream": payload.Upstream,
			"action":   payload.Action,
		})
	})

	// Upstream routes
	r.Mount("/api/catalog", pools["catalog"])
	r.Mount("/api/search", pools["search"])
	r.Mount("/api/cart", pools["cart"])
	r.Mount("/api/checkout", pools["checkout"])
	r.Mount("/api/orders", pools["orders"])
	r.Mount("/api/inventory", pools["inventory"])
	r.Mount("/api/payments", pools["payments"])
	r.Mount("/api/reviews", pools["reviews"])

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
