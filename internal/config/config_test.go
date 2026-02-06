package config

import (
	"testing"
	"time"
)

func TestLoadEnablesDemoModeWhenBaseURLIsMissing(t *testing.T) {
	t.Setenv("SYNCTHING_BASE_URL", "")
	t.Setenv("SYNCTHING_API_KEY", "")
	t.Setenv("SYNCTHING_API_KEY_FILE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error in demo mode: %v", err)
	}

	if !cfg.DemoMode {
		t.Fatalf("expected DemoMode to be true")
	}
	if cfg.STBaseURL != "" {
		t.Fatalf("expected STBaseURL to be empty in demo mode")
	}
	if cfg.STAPIKey != "" {
		t.Fatalf("expected STAPIKey to be empty in demo mode")
	}
}

func TestLoadRequiresAPIKeyWhenBaseURLIsSet(t *testing.T) {
	t.Setenv("SYNCTHING_BASE_URL", "http://localhost:8384")
	t.Setenv("SYNCTHING_API_KEY", "")
	t.Setenv("SYNCTHING_API_KEY_FILE", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error when API key is missing")
	}
}

func TestLoadReadsSyncthingConfigWhenProvided(t *testing.T) {
	t.Setenv("SYNCTHING_BASE_URL", "http://localhost:8384")
	t.Setenv("SYNCTHING_API_KEY", "demo-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DemoMode {
		t.Fatalf("expected DemoMode to be false")
	}
	if cfg.STBaseURL != "http://localhost:8384" {
		t.Fatalf("unexpected STBaseURL: %q", cfg.STBaseURL)
	}
	if cfg.STAPIKey != "demo-key" {
		t.Fatalf("unexpected STAPIKey")
	}
}

func TestLoadAcceptsNumericPollIntervalInSeconds(t *testing.T) {
	t.Setenv("SYNCTHING_BASE_URL", "")
	t.Setenv("SYNCTHING_DASHBOARD_POLL_INTERVAL", "2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.PollInterval != 2*time.Second {
		t.Fatalf("expected poll interval of 2s, got %s", cfg.PollInterval)
	}
}
