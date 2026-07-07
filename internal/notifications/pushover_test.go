package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

func TestPushoverNotifierPostsFormPayload(t *testing.T) {
	var (
		gotMethod string
		gotForm   url.Values
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := newPushoverNotifier(config.PushoverNotificationConfig{
		Token:    "app-token",
		UserKey:  "user-key",
		Device:   "iphone",
		Sound:    "pushover",
		Priority: 1,
		Timeout:  config.Duration{Duration: time.Second},
	}, server.URL)

	err := notifier.RecordingSubmitted(context.Background(), plex.RecordingRequest{
		RuleName:    "Formula 1",
		Title:       "Formula 1",
		Subtitle:    "British Grand Prix: Race",
		ChannelName: "Sky Sports F1 HD",
		StartAt:     time.Date(2026, 7, 5, 18, 40, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordingSubmitted() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if got, want := gotForm.Get("token"), "app-token"; got != want {
		t.Fatalf("token = %q, want %q", got, want)
	}
	if got, want := gotForm.Get("user"), "user-key"; got != want {
		t.Fatalf("user = %q, want %q", got, want)
	}
	if got, want := gotForm.Get("device"), "iphone"; got != want {
		t.Fatalf("device = %q, want %q", got, want)
	}
	if got, want := gotForm.Get("sound"), "pushover"; got != want {
		t.Fatalf("sound = %q, want %q", got, want)
	}
	if got, want := gotForm.Get("priority"), "1"; got != want {
		t.Fatalf("priority = %q, want %q", got, want)
	}
	if got := gotForm.Get("message"); got == "" {
		t.Fatal("expected message to be populated")
	}
}
