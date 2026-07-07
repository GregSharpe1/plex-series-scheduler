package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

func TestNewReturnsNopNotifierWhenWebhookDisabled(t *testing.T) {
	notifier, err := New(config.NotificationsConfig{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := notifier.RecordingSubmitted(context.Background(), plex.RecordingRequest{}); err != nil {
		t.Fatalf("RecordingSubmitted() error = %v", err)
	}
}

func TestWebhookNotifierPostsRecordingSubmittedPayload(t *testing.T) {
	type payload struct {
		Event       string    `json:"event"`
		RuleName    string    `json:"ruleName"`
		Title       string    `json:"title"`
		Subtitle    string    `json:"subtitle"`
		ChannelName string    `json:"channelName"`
		AiringID    string    `json:"airingId"`
		StartAt     time.Time `json:"startAt"`
	}

	var (
		gotMethod string
		gotType   string
		gotAuth   string
		gotBody   payload
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	notifier, err := New(config.NotificationsConfig{Webhook: config.WebhookNotificationConfig{
		URL: server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
		Timeout: config.Duration{Duration: time.Second},
	}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	startAt := time.Date(2026, 7, 7, 20, 0, 0, 0, time.UTC)
	err = notifier.RecordingSubmitted(context.Background(), plex.RecordingRequest{
		RuleName:    "Formula 1",
		Title:       "Formula 1",
		Subtitle:    "British Grand Prix: Race",
		ChannelName: "Sky Sports F1 HD",
		AiringID:    "airing-1",
		StartAt:     startAt,
	})
	if err != nil {
		t.Fatalf("RecordingSubmitted() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotType != "application/json" {
		t.Fatalf("content-type = %q, want application/json", gotType)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("authorization = %q, want %q", gotAuth, "Bearer token")
	}
	if gotBody.Event != "recording_submitted" {
		t.Fatalf("event = %q, want recording_submitted", gotBody.Event)
	}
	if gotBody.RuleName != "Formula 1" {
		t.Fatalf("ruleName = %q, want Formula 1", gotBody.RuleName)
	}
	if gotBody.Title != "Formula 1" {
		t.Fatalf("title = %q, want Formula 1", gotBody.Title)
	}
	if gotBody.Subtitle != "British Grand Prix: Race" {
		t.Fatalf("subtitle = %q, want British Grand Prix: Race", gotBody.Subtitle)
	}
	if gotBody.ChannelName != "Sky Sports F1 HD" {
		t.Fatalf("channelName = %q, want Sky Sports F1 HD", gotBody.ChannelName)
	}
	if gotBody.AiringID != "airing-1" {
		t.Fatalf("airingId = %q, want airing-1", gotBody.AiringID)
	}
	if !gotBody.StartAt.Equal(startAt) {
		t.Fatalf("startAt = %v, want %v", gotBody.StartAt, startAt)
	}
}
