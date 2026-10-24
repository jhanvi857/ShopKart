package admin

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"sync"
)

type ConfigManager struct {
	mu           sync.RWMutex
	adminKey     string
	faultEnabled bool
	settings     map[string]interface{}
}

func NewConfigManager(adminKey string, faultEndpoints bool) *ConfigManager {
	return &ConfigManager{
		adminKey:     adminKey,
		faultEnabled: faultEndpoints,
		settings:     make(map[string]interface{}),
	}
}

func (cm *ConfigManager) Set(key string, val interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.settings[key] = val
}

func (cm *ConfigManager) Get(key string) (interface{}, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	val, ok := cm.settings[key]
	return val, ok
}

func (cm *ConfigManager) Authenticate(r *http.Request) bool {
	key := r.Header.Get("X-Admin-Key")
	if key == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return key != "" && key == cm.adminKey
}

func (cm *ConfigManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cm.Authenticate(r) {
			http.Error(w, `{"error":"unauthorized: invalid admin key"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type RuntimeStats struct {
	NumGoroutine int                    `json:"num_goroutine"`
	AllocMB      uint64                 `json:"alloc_mb"`
	TotalAllocMB uint64                 `json:"total_alloc_mb"`
	SysMB        uint64                 `json:"sys_mb"`
	NumGC        uint32                 `json:"num_gc"`
	Settings     map[string]interface{} `json:"settings"`
	FaultEnabled bool                   `json:"fault_endpoints_enabled"`
}

func (cm *ConfigManager) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if !cm.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cm.mu.RLock()
	settingsCopy := make(map[string]interface{}, len(cm.settings))
	for k, v := range cm.settings {
		settingsCopy[k] = v
	}
	cm.mu.RUnlock()

	stats := RuntimeStats{
		NumGoroutine: runtime.NumGoroutine(),
		AllocMB:      m.Alloc / 1024 / 1024,
		TotalAllocMB: m.TotalAlloc / 1024 / 1024,
		SysMB:        m.Sys / 1024 / 1024,
		NumGC:        m.NumGC,
		Settings:     settingsCopy,
		FaultEnabled: cm.faultEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (cm *ConfigManager) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !cm.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	cm.mu.Lock()
	for k, v := range updates {
		cm.settings[k] = v
	}
	cm.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "updated",
		"updated": updates,
	})
}
