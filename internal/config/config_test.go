package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaultsAndEnvExpansion(t *testing.T) {
	t.Setenv("PLEX_TOKEN", "secret-token")
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: ${PLEX_TOKEN}

scheduler:
  dryRun: true

rules:
  - name: Formula 1
    enabled: true
    matchMode: sports_event
    titleRegex: "^Formula 1"
    includeKeywords:
      - Grand Prix
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Plex.Token != "secret-token" {
		t.Fatalf("expected expanded token, got %q", cfg.Plex.Token)
	}
	if got, want := cfg.Scheduler.Interval.Duration, 30*time.Minute; got != want {
		t.Fatalf("interval = %v, want %v", got, want)
	}
	if got, want := cfg.Scheduler.GuideLookahead.Duration, 7*24*time.Hour; got != want {
		t.Fatalf("guideLookahead = %v, want %v", got, want)
	}
	if cfg.Rules[0].CompiledTitleRegex() == nil {
		t.Fatal("expected compiled title regex")
	}
}

func TestLoadRejectsInvalidRule(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

scheduler:
  interval: 30m

rules:
  - name: Broken
    matchMode: regex
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadRejectsNegativeMaxRecordings(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

scheduler:
  interval: 30m
  maxRecordings: -1

rules:
  - name: Limited
    matchMode: contains
    title: Formula 1
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadAppliesDefaultWebhookTimeout(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

notifications:
  webhook:
    url: https://example.com/hooks/plex

rules:
  - name: Taskmaster
    matchMode: exact
    title: Taskmaster
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Notifications.Webhook.Timeout.Duration, 10*time.Second; got != want {
		t.Fatalf("webhook timeout = %v, want %v", got, want)
	}
}

func TestLoadRejectsInvalidWebhookURL(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

notifications:
  webhook:
    url: ftp://example.com/hooks/plex

rules:
  - name: Taskmaster
    matchMode: exact
    title: Taskmaster
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadAppliesDefaultPushoverTimeout(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

notifications:
  pushover:
    token: app-token
    userKey: user-key

rules:
  - name: Taskmaster
    matchMode: exact
    title: Taskmaster
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Notifications.Pushover.Timeout.Duration, 10*time.Second; got != want {
		t.Fatalf("pushover timeout = %v, want %v", got, want)
	}
}

func TestLoadRejectsIncompletePushoverConfig(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

notifications:
  pushover:
    token: app-token

rules:
  - name: Taskmaster
    matchMode: exact
    title: Taskmaster
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadRejectsInvalidPushoverPriority(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	content := []byte(`plex:
  url: http://plex:32400
  token: token

notifications:
  pushover:
    token: app-token
    userKey: user-key
    priority: 3

rules:
  - name: Taskmaster
    matchMode: exact
    title: Taskmaster
`)

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}
