package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var envOnce sync.Once

func initEnv() {
	envOnce.Do(func() {
		// Look for .env in current, parent, or grandparent dir
		candidates := []string{".env", "../.env", "../../.env"}
		for _, path := range candidates {
			abs, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			f, err := os.Open(abs)
			if err == nil {
				defer f.Close()
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						k := strings.TrimSpace(parts[0])
						v := strings.TrimSpace(parts[1])
						if _, exists := os.LookupEnv(k); !exists {
							_ = os.Setenv(k, v)
						}
					}
				}
				break
			}
		}
	})
}

type CommonConfig struct {
	ServiceName     string
	InstanceID      string
	Environment     string
	Port            string
	AdminAPIKey     string
	OTLPEndpoint    string
	FaultEndpoints  bool
	TracingEnabled  bool
	KafkaEnabled    bool
	ShutdownTimeout time.Duration
}

func LoadCommon(serviceName, defaultPort string) CommonConfig {
	initEnv()
	return CommonConfig{
		ServiceName:     GetEnv("SERVICE_NAME", serviceName),
		InstanceID:      GetEnv("INSTANCE_ID", GetHostname()),
		Environment:     GetEnv("ENVIRONMENT", "production"),
		Port:            GetEnv("PORT", defaultPort),
		AdminAPIKey:     GetEnv("ADMIN_API_KEY", "shopkart-ops-secret-key-prod-998"),
		OTLPEndpoint:    GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		FaultEndpoints:  GetEnvBool("SHOPKART_FAULT_ENDPOINTS", true),
		TracingEnabled:  GetEnvBool("TRACING_ENABLED", true),
		KafkaEnabled:    GetEnvBool("KAFKA_ENABLED", true),
		ShutdownTimeout: 5 * time.Second,
	}
}

func GetEnv(key, defaultVal string) string {
	initEnv()
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func GetEnvInt(key string, defaultVal int) int {
	initEnv()
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

func GetEnvBool(key string, defaultVal bool) bool {
	initEnv()
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

func GetEnvDuration(key string, defaultVal time.Duration) time.Duration {
	initEnv()
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func GetHostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "instance-1"
	}
	return name
}
