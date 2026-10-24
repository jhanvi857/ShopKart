package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY", "custom_value")
	defer os.Unsetenv("TEST_KEY")

	if val := GetEnv("TEST_KEY", "default"); val != "custom_value" {
		t.Errorf("expected custom_value, got %s", val)
	}

	if val := GetEnv("NON_EXISTENT", "default"); val != "default" {
		t.Errorf("expected default, got %s", val)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if val := GetEnvInt("TEST_INT", 10); val != 42 {
		t.Errorf("expected 42, got %d", val)
	}

	if val := GetEnvInt("INVALID_INT", 10); val != 10 {
		t.Errorf("expected default 10, got %d", val)
	}
}

func TestGetEnvBool(t *testing.T) {
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")

	if val := GetEnvBool("TEST_BOOL", false); !val {
		t.Errorf("expected true, got false")
	}

	if val := GetEnvBool("NON_EXISTENT", true); !val {
		t.Errorf("expected true, got false")
	}
}

func TestLoadCommon(t *testing.T) {
	cfg := LoadCommon("my-test-service", "8080")
	if cfg.ServiceName != "my-test-service" {
		t.Errorf("expected my-test-service, got %s", cfg.ServiceName)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected 8080, got %s", cfg.Port)
	}
}
