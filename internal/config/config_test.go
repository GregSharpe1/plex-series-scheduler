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
