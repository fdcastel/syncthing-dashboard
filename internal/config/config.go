package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config stores runtime configuration for the dashboard service.
type Config struct {
	STBaseURL            string
	STAPIKey             string
	DemoMode             bool
	PollInterval         time.Duration
	HTTPListenAddr       string
	HTTPReadTimeout      time.Duration
	HTTPWriteTimeout     time.Duration
	STTimeout            time.Duration
	STInsecureSkipVerify bool
	PageTitle            string
	PageSubtitle         string
}

// Load reads environment variables and validates required settings.
func Load() (Config, error) {
	baseURL := strings.TrimSpace(os.Getenv("SYNCTHING_BASE_URL"))

	pollInterval, err := durationFromEnv("SYNCTHING_DASHBOARD_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	if pollInterval <= 0 {
		return Config{}, fmt.Errorf("SYNCTHING_DASHBOARD_POLL_INTERVAL must be > 0")
	}

	httpReadTimeout, err := durationFromEnv("SYNCTHING_DASHBOARD_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	if httpReadTimeout <= 0 {
		return Config{}, fmt.Errorf("SYNCTHING_DASHBOARD_READ_TIMEOUT must be > 0")
	}

	httpWriteTimeout, err := durationFromEnv("SYNCTHING_DASHBOARD_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	if httpWriteTimeout <= 0 {
		return Config{}, fmt.Errorf("SYNCTHING_DASHBOARD_WRITE_TIMEOUT must be > 0")
	}

	stTimeout, err := durationFromEnv("SYNCTHING_TIMEOUT", 8*time.Second)
	if err != nil {
		return Config{}, err
	}
	if stTimeout <= 0 {
		return Config{}, fmt.Errorf("SYNCTHING_TIMEOUT must be > 0")
	}

	stInsecureSkipVerify, err := boolFromEnv("SYNCTHING_INSECURE_SKIP_VERIFY", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DemoMode:             baseURL == "",
		PollInterval:         pollInterval,
		HTTPListenAddr:       stringFromEnv("SYNCTHING_DASHBOARD_LISTEN_ADDRESS", ":8080"),
		HTTPReadTimeout:      httpReadTimeout,
		HTTPWriteTimeout:     httpWriteTimeout,
		STTimeout:            stTimeout,
		STInsecureSkipVerify: stInsecureSkipVerify,
		PageTitle:            stringFromEnv("SYNCTHING_DASHBOARD_TITLE", "Syncthing"),
		PageSubtitle:         stringFromEnv("SYNCTHING_DASHBOARD_SUBTITLE", "Read-Only Dashboard"),
	}

	if cfg.DemoMode {
		return cfg, nil
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Config{}, fmt.Errorf("SYNCTHING_BASE_URL must be a valid absolute URL")
	}

	apiKey, err := loadAPIKey()
	if err != nil {
		return Config{}, err
	}

	cfg.STBaseURL = strings.TrimRight(parsedURL.String(), "/")
	cfg.STAPIKey = apiKey

	return cfg, nil
}

func loadAPIKey() (string, error) {
	if apiKey := strings.TrimSpace(os.Getenv("SYNCTHING_API_KEY")); apiKey != "" {
		return apiKey, nil
	}

	secretPath := strings.TrimSpace(os.Getenv("SYNCTHING_API_KEY_FILE"))
	if secretPath == "" {
		return "", fmt.Errorf("either SYNCTHING_API_KEY or SYNCTHING_API_KEY_FILE must be set")
	}

	secretData, err := os.ReadFile(secretPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SYNCTHING_API_KEY_FILE: %w", err)
	}

	apiKey := strings.TrimSpace(string(secretData))
	if apiKey == "" {
		return "", fmt.Errorf("SYNCTHING_API_KEY_FILE is empty")
	}

	return apiKey, nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err == nil {
		return parsed, nil
	}

	// Accept plain integers as seconds for convenience (e.g. "2" => 2s).
	if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, nil
	}

	return 0, fmt.Errorf("%s: invalid duration %q", name, value)
}

func boolFromEnv(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: invalid boolean %q", name, value)
	}

	return parsed, nil
}

func stringFromEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	return value
}
