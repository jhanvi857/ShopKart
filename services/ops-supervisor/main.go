package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
)

var ServiceAllowlist = map[string]int{
	"edge-gateway":       9000,
	"catalog":            9001,
	"search":             9002,
	"checkout":           9003,
	"inventory":          9004,
	"payments":           9005,
	"cart":               9006,
	"notifications":      9007,
	"external-bank-mock": 9008,
	"reviews":            9009,
}

type Instance struct {
	Service       string    `json:"service"`
	Port          int       `json:"port"`
	PID           int       `json:"pid"`
	StartTime     time.Time `json:"start_time"`
	UptimeSeconds float64   `json:"uptime_seconds"`
	Status        string    `json:"status"`
	Cmd           *exec.Cmd `json:"-"`
}

type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	ClientIP  string    `json:"client_ip"`
	Action    string    `json:"action"`
	Service   string    `json:"service,omitempty"`
	Port      int       `json:"port,omitempty"`
	PID       int       `json:"pid,omitempty"`
	Details   string    `json:"details,omitempty"`
	Outcome   string    `json:"outcome"`
}

type Supervisor struct {
	instMu      sync.RWMutex
	auditMu     sync.Mutex
	adminKey    string
	instances   map[string]*Instance // key: "service:port"
	auditLog    []AuditEntry
	auditFile   *os.File
	servicesDir string
}

func NewSupervisor(adminKey string, auditLogPath string) (*Supervisor, error) {
	servicesDir := "."
	if _, err := os.Stat("services/edge-gateway"); err == nil {
		servicesDir = "services"
	} else if _, err := os.Stat("../edge-gateway"); err == nil {
		servicesDir = ".."
	}

	var f *os.File
	if auditLogPath != "" {
		_ = os.MkdirAll(filepath.Dir(auditLogPath), 0755)
		file, err := os.OpenFile(auditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			f = file
		}
	}

	return &Supervisor{
		adminKey:    adminKey,
		instances:   make(map[string]*Instance),
		auditLog:    make([]AuditEntry, 0),
		auditFile:   f,
		servicesDir: servicesDir,
	}, nil
}

func (s *Supervisor) Audit(r *http.Request, action, service string, port, pid int, details, outcome string) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	clientIP := ""
	if r != nil {
		clientIP = r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
		}
	}

	entry := AuditEntry{
		Timestamp: time.Now().UTC(),
		ClientIP:  clientIP,
		Action:    action,
		Service:   service,
		Port:      port,
		PID:       pid,
		Details:   details,
		Outcome:   outcome,
	}

	s.auditLog = append(s.auditLog, entry)
	if len(s.auditLog) > 1000 {
		s.auditLog = s.auditLog[len(s.auditLog)-1000:]
	}

	slog.Info("OPS_AUDIT",
		slog.String("action", action),
		slog.String("service", service),
		slog.Int("port", port),
		slog.Int("pid", pid),
		slog.String("outcome", outcome),
		slog.String("client_ip", clientIP),
	)

	if s.auditFile != nil {
		b, err := json.Marshal(entry)
		if err == nil {
			_, _ = s.auditFile.Write(append(b, '\n'))
		}
	}
}

func (s *Supervisor) Authenticate(r *http.Request) bool {
	key := r.Header.Get("X-Admin-Key")
	if key == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return key != "" && key == s.adminKey
}

func (s *Supervisor) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Authenticate(r) {
			s.Audit(r, "AUTHENTICATE", "", 0, 0, r.URL.Path, "UNAUTHORIZED")
			http.Error(w, `{"error":"unauthorized: invalid admin key"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// FindAllListeningPIDs finds all TCP listening ports and their PIDs in one fast scan
func FindAllListeningPIDs() (map[int]int, error) {
	result := make(map[int]int)
	if runtime.GOOS == "windows" {
		cmd := exec.Command("netstat", "-ano", "-p", "tcp")
		out, err := cmd.Output()
		if err != nil {
			return result, err
		}
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.Contains(line, "LISTENING") {
				fields := strings.Fields(line)
				if len(fields) >= 5 {
					addr := fields[1]
					idx := strings.LastIndex(addr, ":")
					if idx != -1 {
						port, err := strconv.Atoi(addr[idx+1:])
						pid, err2 := strconv.Atoi(fields[len(fields)-1])
						if err == nil && err2 == nil && port > 0 && pid > 0 {
							result[port] = pid
						}
					}
				}
			}
		}
	} else {
		cmd := exec.Command("sh", "-c", "lsof -iTCP -sTCP:LISTEN -n -P 2>/dev/null || ss -lptn 2>/dev/null")
		out, err := cmd.Output()
		if err == nil {
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				fields := strings.Fields(line)
				if len(fields) >= 9 && strings.Contains(fields[0], "COMMAND") == false {
					// lsof output format: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME (LISTEN)
					if pid, err := strconv.Atoi(fields[1]); err == nil {
						nameField := fields[8]
						idx := strings.LastIndex(nameField, ":")
						if idx != -1 {
							if port, err2 := strconv.Atoi(nameField[idx+1:]); err2 == nil {
								result[port] = pid
							}
						}
					}
				}
			}
		}
	}
	return result, nil
}

// FindListeningPID finds the PID listening on a given port across platforms
func FindListeningPID(port int) (int, error) {
	pids, err := FindAllListeningPIDs()
	if err != nil {
		return 0, err
	}
	if pid, ok := pids[port]; ok && pid > 0 {
		return pid, nil
	}
	return 0, fmt.Errorf("no process found listening on port %d", port)
}

func (s *Supervisor) ListInstances(w http.ResponseWriter, r *http.Request) {
	s.instMu.Lock()
	defer s.instMu.Unlock()

	// Update active instances in a single fast port scan
	portPIDs, _ := FindAllListeningPIDs()
	results := make([]Instance, 0)
	now := time.Now()

	for svc, defaultPort := range ServiceAllowlist {
		key := fmt.Sprintf("%s:%d", svc, defaultPort)
		inst, exists := s.instances[key]

		pid, ok := portPIDs[defaultPort]
		if ok && pid > 0 {
			if !exists || inst.PID != pid {
				inst = &Instance{
					Service:   svc,
					Port:      defaultPort,
					PID:       pid,
					StartTime: now,
					Status:    "running",
				}
				s.instances[key] = inst
			}
			inst.UptimeSeconds = now.Sub(inst.StartTime).Seconds()
			results = append(results, *inst)
		} else if exists {
			delete(s.instances, key)
		}
	}

	// Also include any extra port instances (e.g. replicas on 9013)
	for key, inst := range s.instances {
		isDefault := false
		for _, defPort := range ServiceAllowlist {
			if inst.Port == defPort {
				isDefault = true
				break
			}
		}
		if !isDefault {
			pid, ok := portPIDs[inst.Port]
			if ok && pid > 0 {
				inst.PID = pid
				inst.UptimeSeconds = now.Sub(inst.StartTime).Seconds()
				results = append(results, *inst)
			} else {
				delete(s.instances, key)
			}
		}
	}

	s.Audit(r, "LIST_INSTANCES", "", 0, 0, fmt.Sprintf("%d instances running", len(results)), "SUCCESS")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (s *Supervisor) StartInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
		Port    int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Audit(r, "START_INSTANCE", "", 0, 0, "invalid body", "ERROR")
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	if _, allowed := ServiceAllowlist[req.Service]; !allowed {
		s.Audit(r, "START_INSTANCE", req.Service, req.Port, 0, "service not in allowlist", "FORBIDDEN")
		http.Error(w, fmt.Sprintf(`{"error":"service %s is not in allowlist"}`, req.Service), http.StatusForbidden)
		return
	}

	if req.Port <= 0 {
		req.Port = ServiceAllowlist[req.Service]
	}

	// Check if already running on this port
	if pid, err := FindListeningPID(req.Port); err == nil && pid > 0 {
		s.Audit(r, "START_INSTANCE", req.Service, req.Port, pid, "port already in use", "ERROR")
		http.Error(w, fmt.Sprintf(`{"error":"port %d already occupied by pid %d"}`, req.Port, pid), http.StatusConflict)
		return
	}

	// Configure environment
	prefix := strings.ToUpper(strings.ReplaceAll(req.Service, "-", "_"))
	procs := config.GetEnv(prefix+"_GOMAXPROCS", config.GetEnv("GOMAXPROCS", "2"))
	mem := config.GetEnv(prefix+"_GOMEMLIMIT", config.GetEnv("GOMEMLIMIT", "256MiB"))

	cmd := exec.Command("go", "run", "./"+req.Service)
	cmd.Dir = s.servicesDir

	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	envMap["PORT"] = strconv.Itoa(req.Port)
	envMap["GOMAXPROCS"] = procs
	envMap["GOMEMLIMIT"] = mem

	customEnv := make([]string, 0, len(envMap))
	for k, v := range envMap {
		customEnv = append(customEnv, k+"="+v)
	}
	cmd.Env = customEnv

	if err := cmd.Start(); err != nil {
		s.Audit(r, "START_INSTANCE", req.Service, req.Port, 0, err.Error(), "FAILED")
		http.Error(w, fmt.Sprintf(`{"error":"failed to start instance: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	inst := &Instance{
		Service:   req.Service,
		Port:      req.Port,
		PID:       cmd.Process.Pid,
		StartTime: time.Now(),
		Status:    "running",
		Cmd:       cmd,
	}

	key := fmt.Sprintf("%s:%d", req.Service, req.Port)
	s.instMu.Lock()
	s.instances[key] = inst
	s.instMu.Unlock()

	// Notify edge-gateway dynamically if edge-gateway is reachable
	go s.notifyGatewayUpstream(req.Service, fmt.Sprintf("localhost:%d", req.Port), "add")

	s.Audit(r, "START_INSTANCE", req.Service, req.Port, cmd.Process.Pid, fmt.Sprintf("GOMAXPROCS=%s GOMEMLIMIT=%s", procs, mem), "SUCCESS")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(inst)
}

func (s *Supervisor) StopInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
		Port    int    `json:"port"`
		PID     int    `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Audit(r, "STOP_INSTANCE", "", 0, 0, "invalid body", "ERROR")
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	targetPID := req.PID
	if targetPID <= 0 && req.Port > 0 {
		pid, err := FindListeningPID(req.Port)
		if err == nil {
			targetPID = pid
		}
	}

	if targetPID <= 0 {
		s.Audit(r, "STOP_INSTANCE", req.Service, req.Port, 0, "no matching process found", "ERROR")
		http.Error(w, `{"error":"no matching process found to stop"}`, http.StatusNotFound)
		return
	}

	// Terminate process
	proc, err := os.FindProcess(targetPID)
	if err != nil {
		s.Audit(r, "STOP_INSTANCE", req.Service, req.Port, targetPID, err.Error(), "ERROR")
		http.Error(w, fmt.Sprintf(`{"error":"failed to find process: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = proc.Kill()

	if req.Port > 0 {
		key := fmt.Sprintf("%s:%d", req.Service, req.Port)
		s.instMu.Lock()
		delete(s.instances, key)
		s.instMu.Unlock()

		// Notify edge-gateway to remove upstream
		go s.notifyGatewayUpstream(req.Service, fmt.Sprintf("localhost:%d", req.Port), "remove")
	}

	s.Audit(r, "STOP_INSTANCE", req.Service, req.Port, targetPID, "killed process", "SUCCESS")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "stopped",
		"service": req.Service,
		"port":    req.Port,
		"pid":     targetPID,
	})
}

func (s *Supervisor) RestartInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
		Port    int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Audit(r, "RESTART_INSTANCE", "", 0, 0, "invalid body", "ERROR")
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	if _, allowed := ServiceAllowlist[req.Service]; !allowed {
		s.Audit(r, "RESTART_INSTANCE", req.Service, req.Port, 0, "service not in allowlist", "FORBIDDEN")
		http.Error(w, fmt.Sprintf(`{"error":"service %s is not in allowlist"}`, req.Service), http.StatusForbidden)
		return
	}

	if req.Port <= 0 {
		req.Port = ServiceAllowlist[req.Service]
	}

	// Stop existing process if running
	if pid, err := FindListeningPID(req.Port); err == nil && pid > 0 {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Start new instance
	startBody, _ := json.Marshal(req)
	subReq, _ := http.NewRequest(http.MethodPost, "/ops/instances/start", bytes.NewReader(startBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq.Header.Set("X-Admin-Key", s.adminKey)

	s.StartInstance(w, subReq)
}

func (s *Supervisor) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string                 `json:"service"`
		Port    int                    `json:"port"`
		Config  map[string]interface{} `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Audit(r, "UPDATE_CONFIG", "", 0, 0, "invalid body", "ERROR")
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	if _, allowed := ServiceAllowlist[req.Service]; !allowed {
		s.Audit(r, "UPDATE_CONFIG", req.Service, req.Port, 0, "service not in allowlist", "FORBIDDEN")
		http.Error(w, fmt.Sprintf(`{"error":"service %s is not in allowlist"}`, req.Service), http.StatusForbidden)
		return
	}

	if req.Port <= 0 {
		req.Port = ServiceAllowlist[req.Service]
	}

	targetURL := fmt.Sprintf("http://localhost:%d/admin/config", req.Port)
	bodyBytes, _ := json.Marshal(req.Config)

	proxyReq, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		s.Audit(r, "UPDATE_CONFIG", req.Service, req.Port, 0, err.Error(), "FAILED")
		http.Error(w, fmt.Sprintf(`{"error":"failed to create upstream request: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("X-Admin-Key", s.adminKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		s.Audit(r, "UPDATE_CONFIG", req.Service, req.Port, 0, err.Error(), "FAILED")
		http.Error(w, fmt.Sprintf(`{"error":"upstream admin request failed: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	s.Audit(r, "UPDATE_CONFIG", req.Service, req.Port, 0, string(bodyBytes), fmt.Sprintf("UPSTREAM_STATUS_%d", resp.StatusCode))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBytes)
}

func (s *Supervisor) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.auditLog)
}

func (s *Supervisor) notifyGatewayUpstream(service, upstream, action string) {
	gatewayURL := "http://localhost:9000/admin/upstreams"
	body, _ := json.Marshal(map[string]string{
		"service":  service,
		"upstream": upstream,
		"action":   action,
	})
	req, err := http.NewRequest(http.MethodPost, gatewayURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", s.adminKey)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func main() {
	cfg := config.LoadCommon("ops-supervisor", "9010")
	logger := logging.InitLogger(cfg.ServiceName, cfg.InstanceID, cfg.Environment)

	auditLogPath := config.GetEnv("OPS_AUDIT_LOG", "docs/ops-audit.log")
	sup, err := NewSupervisor(cfg.AdminAPIKey, auditLogPath)
	if err != nil {
		logger.Error("Failed to init supervisor", slog.String("error", err.Error()))
		os.Exit(1)
	}

	checker := health.NewChecker()
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())

	// Authenticated Ops API routes
	r.Group(func(op chi.Router) {
		op.Use(sup.AuthMiddleware)

		op.Get("/ops/instances", sup.ListInstances)
		op.Post("/ops/instances/start", sup.StartInstance)
		op.Post("/ops/instances/stop", sup.StopInstance)
		op.Post("/ops/instances/restart", sup.RestartInstance)
		op.Post("/ops/config", sup.UpdateConfig)
		op.Get("/ops/audit", sup.GetAuditLog)
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("Starting ShopKart Local Ops Supervisor", slog.String("port", cfg.Port))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
