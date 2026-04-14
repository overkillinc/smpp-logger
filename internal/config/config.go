package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddr      = ":2775"
	defaultSystemID        = "smpp-logger"
	defaultPassword        = "smpp-logger"
	defaultLogFormat       = "text"
	defaultShutdownTimeout = 10 * time.Second

	// HTTP UI defaults
	defaultUIAddr  = ":8080"
	defaultUIUser  = "admin"
	defaultUIPass  = "admin"
)

type Config struct {
	ListenAddr      string
	SystemID        string
	Password        string
	LogFormat       string
	ShutdownTimeout time.Duration

	// UI settings
	UIAddr string
	UIUser string
	UIPass string
}

func Load() (Config, error) {
	return LoadFromEnv(os.LookupEnv)
}

func LoadFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{
		ListenAddr:      defaultListenAddr,
		SystemID:        defaultSystemID,
		Password:        defaultPassword,
		LogFormat:       defaultLogFormat,
		ShutdownTimeout: defaultShutdownTimeout,
	}

	if value, ok := lookup("SMPP_LOGGER_LISTEN_ADDR"); ok {
		cfg.ListenAddr = strings.TrimSpace(value)
	}
	if value, ok := lookup("SMPP_LOGGER_SYSTEM_ID"); ok {
		cfg.SystemID = strings.TrimSpace(value)
	}
	if value, ok := lookup("SMPP_LOGGER_PASSWORD"); ok {
		cfg.Password = strings.TrimSpace(value)
	}
	if value, ok := lookup("SMPP_LOGGER_LOG_FORMAT"); ok {
		cfg.LogFormat = strings.TrimSpace(strings.ToLower(value))
	}
	if value, ok := lookup("SMPP_LOGGER_SHUTDOWN_TIMEOUT"); ok {
		timeout, err := time.ParseDuration(strings.TrimSpace(value))
		if err != nil {
			return Config{}, fmt.Errorf("parse SMPP_LOGGER_SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = timeout
	}

	// UI settings
	if value, ok := lookup("SMPP_LOGGER_UI_ADDR"); ok {
		cfg.UIAddr = strings.TrimSpace(value)
	} else {
		cfg.UIAddr = defaultUIAddr
	}
	if value, ok := lookup("SMPP_LOGGER_UI_USER"); ok {
		cfg.UIUser = strings.TrimSpace(value)
	} else {
		cfg.UIUser = defaultUIUser
	}
	if value, ok := lookup("SMPP_LOGGER_UI_PASS"); ok {
		cfg.UIPass = strings.TrimSpace(value)
	} else {
		cfg.UIPass = defaultUIPass
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	switch {
	case c.ListenAddr == "":
		return errors.New("SMPP_LOGGER_LISTEN_ADDR must not be empty")
	case c.SystemID == "":
		return errors.New("SMPP_LOGGER_SYSTEM_ID must not be empty")
	case c.Password == "":
		return errors.New("SMPP_LOGGER_PASSWORD must not be empty")
	case c.ShutdownTimeout <= 0:
		return errors.New("SMPP_LOGGER_SHUTDOWN_TIMEOUT must be greater than zero")
	}

	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("unsupported SMPP_LOGGER_LOG_FORMAT %q", c.LogFormat)
	}

	return nil
}
