package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/admin"
	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type CheckoutApp struct {
	db           *sql.DB
	cartURL      string
	inventoryURL string
	paymentsURL  string
	kafkaWriter  *kafka.Writer
	httpClient   *http.Client
	configMgr    *admin.ConfigManager
	tracer       trace.Tracer
	metrics      *metrics.Metrics
	workerSem    chan struct{}
}

type CheckoutRequest struct {
	CartID          string  `json:"cart_id"`
	CustomerName    string  `json:"customer_name"`
	CustomerEmail   string  `json:"customer_email"`
	ShippingAddress string  `json:"shipping_address"`
	CardToken       string  `json:"card_token"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type Order struct {
	OrderID         string      `json:"order_id"`
	CustomerName    string      `json:"customer_name"`
	CustomerEmail   string      `json:"customer_email"`
	ShippingAddress string      `json:"shipping_address"`
	TotalAmount     float64     `json:"total_amount"`
	Currency        string      `json:"currency"`
	PaymentID       string      `json:"payment_id"`
	TransactionID   string      `json:"transaction_id"`
	Status          string      `json:"status"`
	Items           []OrderItem `json:"items"`
	CreatedAt       time.Time   `json:"created_at"`
}

func main() {
	cfg := config.LoadCommon("checkout", "9003")
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

	dbConnStr := config.GetEnv("DATABASE_URL", "postgres://shopkart_user:shopkart_pass_secret@localhost:5433/shopkart_db?sslmode=disable")
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		logger.Error("Failed to open DB", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	initialMaxConns := config.GetEnvInt("DB_MAX_OPEN_CONNS", 10)
	db.SetMaxOpenConns(initialMaxConns)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	stopDBMon := make(chan struct{})
	defer close(stopDBMon)
	m.MonitorDB(db, "checkout_db", 2*time.Second, stopDBMon)

	var kafkaWriter *kafka.Writer
	if cfg.KafkaEnabled {
		kafkaBrokers := []string{config.GetEnv("KAFKA_BROKER", "localhost:9094")}
		kafkaWriter = &kafka.Writer{
			Addr:         kafka.TCP(kafkaBrokers...),
			Topic:        "shopkart-orders",
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafka.RequireOne,
		}
		defer kafkaWriter.Close()
	}

	workerLimit := config.GetEnvInt("CHECKOUT_WORKER_LIMIT", 20)

	app := &CheckoutApp{
		db:           db,
		cartURL:      config.GetEnv("CART_URL", "http://localhost:9006"),
		inventoryURL: config.GetEnv("INVENTORY_URL", "http://localhost:9004"),
		paymentsURL:  config.GetEnv("PAYMENTS_URL", "http://localhost:9005"),
		kafkaWriter:  kafkaWriter,
		httpClient:   &http.Client{Timeout: 6 * time.Second},
		configMgr:    admin.NewConfigManager(cfg.AdminAPIKey, cfg.FaultEndpoints),
		tracer:       tracer,
		metrics:      m,
		workerSem:    make(chan struct{}, workerLimit),
	}
	app.configMgr.Set("checkout_worker_limit", workerLimit)
	app.configMgr.Set("db_max_open_conns", initialMaxConns)

	checker := health.NewChecker()
	checker.Register("postgres", func(ctx context.Context) error {
		return db.PingContext(ctx)
	})

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(shopkartmw.ObservabilityMiddleware(cfg.ServiceName, m))

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())
	r.Handle("/metrics", m.Handler())

	r.Post("/", app.handleCheckout)
	r.Post("/checkout", app.handleCheckout)
	r.Get("/orders/{orderId}", app.handleGetOrder)

	r.Get("/admin/status", app.configMgr.HandleStatus)
	r.Post("/admin/config", app.handleUpdateConfig)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  12 * time.Second,
		WriteTimeout: 12 * time.Second,
	}

	go func() {
		logger.Info("Starting checkout service", slog.String("port", cfg.Port))
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
	logger.Info("Checkout service shutdown gracefully")
}

func (app *CheckoutApp) handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := app.tracer.Start(ctx, "CheckoutApp.Checkout")
	defer span.End()

	// Acquire concurrency worker slot to reflect real worker saturation
	select {
	case app.workerSem <- struct{}{}:
		defer func() { <-app.workerSem }()
		app.metrics.WorkerPoolUtilization.WithLabelValues("checkout_pool").Set(float64(len(app.workerSem)) / float64(cap(app.workerSem)))
	default:
		app.metrics.WorkerPoolUtilization.WithLabelValues("checkout_pool").Set(1.0)
		http.Error(w, `{"error":"checkout service is at peak capacity, please retry shortly"}`, http.StatusServiceUnavailable)
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CartID == "" {
		http.Error(w, `{"error":"invalid checkout payload"}`, http.StatusBadRequest)
		return
	}

	// 1. Fetch Cart
	cartReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/cart/%s", app.cartURL, req.CartID), nil)
	if err != nil {
		http.Error(w, `{"error":"cart request error"}`, http.StatusInternalServerError)
		return
	}
	tracing.InjectHTTPHeaders(ctx, cartReq)

	cartResp, err := app.httpClient.Do(cartReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"cart service unreachable: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer cartResp.Body.Close()

	var cart struct {
		CartID string      `json:"cart_id"`
		Items  []OrderItem `json:"items"`
		Total  float64     `json:"total"`
	}
	if err := json.NewDecoder(cartResp.Body).Decode(&cart); err != nil || len(cart.Items) == 0 {
		http.Error(w, `{"error":"cart is empty or invalid"}`, http.StatusBadRequest)
		return
	}

	orderID := fmt.Sprintf("ord_%s", uuid.New().String()[:12])
	reservationID := fmt.Sprintf("res_%s", orderID)

	// 2. Reserve inventory for all items
	reservedItems := make([]OrderItem, 0)
	for _, item := range cart.Items {
		reserveBody, _ := json.Marshal(map[string]interface{}{
			"product_id":     item.ProductID,
			"quantity":       item.Quantity,
			"reservation_id": reservationID,
		})
		invReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, app.inventoryURL+"/inventory/reserve", bytes.NewReader(reserveBody))
		invReq.Header.Set("Content-Type", "application/json")
		tracing.InjectHTTPHeaders(ctx, invReq)

		invResp, err := app.httpClient.Do(invReq)
		if err != nil || invResp.StatusCode != http.StatusOK {
			// Rollback already reserved items
			app.rollbackReservations(ctx, reservedItems, reservationID)
			status := http.StatusConflict
			errMsg := "item out of stock"
			if err != nil {
				status = http.StatusBadGateway
				errMsg = "inventory service unavailable"
			}
			http.Error(w, fmt.Sprintf(`{"error":"%s: %s"}`, errMsg, item.ProductID), status)
			return
		}
		invResp.Body.Close()
		reservedItems = append(reservedItems, item)
	}

	// 3. Process payment
	paymentBody, _ := json.Marshal(map[string]interface{}{
		"order_id":        orderID,
		"amount":          cart.Total,
		"currency":        "INR",
		"idempotency_key": fmt.Sprintf("ik_%s", orderID),
		"card_token":      req.CardToken,
	})

	payReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, app.paymentsURL+"/payments/charge", bytes.NewReader(paymentBody))
	payReq.Header.Set("Content-Type", "application/json")
	tracing.InjectHTTPHeaders(ctx, payReq)

	payResp, err := app.httpClient.Do(payReq)
	if err != nil || payResp.StatusCode != http.StatusOK {
		app.rollbackReservations(ctx, reservedItems, reservationID)
		errMsg := "payment declined"
		if err != nil {
			errMsg = "payment service unavailable"
		}
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, errMsg), http.StatusPaymentRequired)
		return
	}

	var payResult struct {
		PaymentID     string `json:"payment_id"`
		TransactionID string `json:"transaction_id"`
		Status        string `json:"status"`
	}
	_ = json.NewDecoder(payResp.Body).Decode(&payResult)
	payResp.Body.Close()

	// 4. Commit Inventory
	for _, item := range reservedItems {
		commitBody, _ := json.Marshal(map[string]interface{}{
			"product_id":     item.ProductID,
			"quantity":       item.Quantity,
			"reservation_id": reservationID,
		})
		cReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, app.inventoryURL+"/inventory/commit", bytes.NewReader(commitBody))
		cReq.Header.Set("Content-Type", "application/json")
		tracing.InjectHTTPHeaders(ctx, cReq)
		cResp, err := app.httpClient.Do(cReq)
		if err == nil {
			cResp.Body.Close()
		}
	}

	// 5. Clear Cart
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/cart/%s", app.cartURL, req.CartID), nil)
	tracing.InjectHTTPHeaders(ctx, delReq)
	delResp, err := app.httpClient.Do(delReq)
	if err == nil {
		delResp.Body.Close()
	}

	// 6. Record order in Postgres
	now := time.Now().UTC()
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO orders (id, customer_name, customer_email, shipping_address, total_amount, currency, payment_id, transaction_id, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		orderID, req.CustomerName, req.CustomerEmail, req.ShippingAddress, cart.Total, "INR", payResult.PaymentID, payResult.TransactionID, "completed", now,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to create order"}`, http.StatusInternalServerError)
		return
	}

	for _, it := range cart.Items {
		_, _ = tx.ExecContext(ctx,
			`INSERT INTO order_items (order_id, product_id, product_name, price, quantity)
			 VALUES ($1, $2, $3, $4, $5)`,
			orderID, it.ProductID, it.Name, it.Price, it.Quantity,
		)
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"commit order failed"}`, http.StatusInternalServerError)
		return
	}

	order := Order{
		OrderID:         orderID,
		CustomerName:    req.CustomerName,
		CustomerEmail:   req.CustomerEmail,
		ShippingAddress: req.ShippingAddress,
		TotalAmount:     cart.Total,
		Currency:        "INR",
		PaymentID:       payResult.PaymentID,
		TransactionID:   payResult.TransactionID,
		Status:          "completed",
		Items:           cart.Items,
		CreatedAt:       now,
	}

	// 7. Publish order placed event to Kafka with W3C tracecontext headers (if Kafka enabled)
	if app.kafkaWriter != nil {
		go func(o Order, traceCtx context.Context) {
			eventData, _ := json.Marshal(o)
			var headers []kafka.Header
			tracing.InjectKafkaHeaders(traceCtx, &headers)

			err := app.kafkaWriter.WriteMessages(context.Background(), kafka.Message{
				Key:     []byte(o.OrderID),
				Value:   eventData,
				Headers: headers,
				Time:    time.Now(),
			})
			if err != nil {
				slog.Warn("Failed to produce order event to Kafka", slog.String("error", err.Error()))
			}
		}(order, ctx)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(order)
}

func (app *CheckoutApp) rollbackReservations(ctx context.Context, items []OrderItem, reservationID string) {
	for _, item := range items {
		body, _ := json.Marshal(map[string]interface{}{
			"product_id":     item.ProductID,
			"quantity":       item.Quantity,
			"reservation_id": reservationID,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, app.inventoryURL+"/inventory/release", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func (app *CheckoutApp) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := chi.URLParam(r, "orderId")

	var order Order
	err := app.db.QueryRowContext(ctx,
		`SELECT id, customer_name, customer_email, shipping_address, total_amount, currency, payment_id, transaction_id, status, created_at
		 FROM orders WHERE id = $1`, orderID,
	).Scan(&order.OrderID, &order.CustomerName, &order.CustomerEmail, &order.ShippingAddress, &order.TotalAmount, &order.Currency, &order.PaymentID, &order.TransactionID, &order.Status, &order.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"order not found"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	rows, err := app.db.QueryContext(ctx,
		`SELECT product_id, product_name, price, quantity FROM order_items WHERE order_id = $1`, orderID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it OrderItem
			if err := rows.Scan(&it.ProductID, &it.Name, &it.Price, &it.Quantity); err == nil {
				order.Items = append(order.Items, it)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(order)
}

func (app *CheckoutApp) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var updates struct {
		DBMaxOpenConns *int `json:"db_max_open_conns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	if updates.DBMaxOpenConns != nil && *updates.DBMaxOpenConns > 0 {
		app.db.SetMaxOpenConns(*updates.DBMaxOpenConns)
		app.configMgr.Set("db_max_open_conns", *updates.DBMaxOpenConns)
	}

	app.configMgr.HandleStatus(w, r)
}
