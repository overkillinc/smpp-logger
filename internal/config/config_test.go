package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadFromEnv(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.ListenAddr != defaultListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, defaultListenAddr)
	}
	if cfg.SystemID != defaultSystemID {
		t.Fatalf("SystemID = %q, want %q", cfg.SystemID, defaultSystemID)
	}
	if cfg.Password != defaultPassword {
		t.Fatalf("Password = %q, want %q", cfg.Password, defaultPassword)
	}
	if cfg.LogFormat != defaultLogFormat {
		t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, defaultLogFormat)
	}
	if cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, defaultShutdownTimeout)
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"SMPP_LOGGER_LISTEN_ADDR":      "0.0.0.0:2775",
		"SMPP_LOGGER_SYSTEM_ID":        "client",
		"SMPP_LOGGER_PASSWORD":         "secret",
		"SMPP_LOGGER_LOG_FORMAT":       "JSON",
		"SMPP_LOGGER_SHUTDOWN_TIMEOUT": "30s",
	}

	cfg, err := LoadFromEnv(func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.ListenAddr != "0.0.0.0:2775" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.SystemID != "client" {
		t.Fatalf("SystemID = %q", cfg.SystemID)
	}
	if cfg.Password != "secret" {
		t.Fatalf("Password = %q", cfg.Password)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("LogFormat = %q", cfg.LogFormat)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
}

func TestLoadFromEnvRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := LoadFromEnv(func(key string) (string, bool) {
		switch key {
		case "SMPP_LOGGER_LOG_FORMAT":
			return "yaml", true
		default:
			return "", false
		}
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported SMPP_LOGGER_LOG_FORMAT") {
		t.Fatalf("LoadFromEnv() error = %v, want unsupported format", err)
	}
}

func TestLoadFromEnvRejectsInvalidDuration(t *testing.T) {
	t.Parallel()

	_, err := LoadFromEnv(func(key string) (string, bool) {
		if key == "SMPP_LOGGER_SHUTDOWN_TIMEOUT" {
			return "later", true
		}
		return "", false
	})
	if err == nil || !strings.Contains(err.Error(), "parse SMPP_LOGGER_SHUTDOWN_TIMEOUT") {
		t.Fatalf("LoadFromEnv() error = %v, want parse failure", err)
	}
}
