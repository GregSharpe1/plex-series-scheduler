package notifications

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

const defaultPushoverAPIURL = "https://api.pushover.net/1/messages.json"

type multiNotifier []Notifier

type pushoverNotifier struct {
	apiURL   string
	token    string
	userKey  string
	device   string
	sound    string
	priority int
	client   *http.Client
}

func (m multiNotifier) RecordingSubmitted(ctx context.Context, req plex.RecordingRequest) error {
	var errs []string
	for _, notifier := range m {
		if err := notifier.RecordingSubmitted(ctx, req); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func newPushoverNotifier(cfg config.PushoverNotificationConfig, apiURL string) Notifier {
	timeout := cfg.Timeout.Duration
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &pushoverNotifier{
		apiURL:   apiURL,
		token:    cfg.Token,
		userKey:  cfg.UserKey,
		device:   cfg.Device,
		sound:    cfg.Sound,
		priority: cfg.Priority,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (n *pushoverNotifier) RecordingSubmitted(ctx context.Context, req plex.RecordingRequest) error {
	form := url.Values{}
	form.Set("token", n.token)
	form.Set("user", n.userKey)
	form.Set("title", "Plex Series Scheduler")
	form.Set("message", formatPushoverMessage(req))
	form.Set("priority", strconv.Itoa(n.priority))
	if n.device != "" {
		form.Set("device", n.device)
	}
	if n.sound != "" {
		form.Set("sound", n.sound)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, n.apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build pushover request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send pushover notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("pushover notification returned status %s", resp.Status)
	}

	return nil
}

func formatPushoverMessage(req plex.RecordingRequest) string {
	parts := []string{req.Title}
	if req.Subtitle != "" {
		parts = append(parts, req.Subtitle)
	} else if req.EpisodeTitle != "" {
		parts = append(parts, req.EpisodeTitle)
	}

	message := fmt.Sprintf("Scheduled recording via rule %q\n%s", req.RuleName, strings.Join(parts, " - "))
	if req.ChannelName != "" {
		message += fmt.Sprintf("\nChannel: %s", req.ChannelName)
	}
	if !req.StartAt.IsZero() {
		message += fmt.Sprintf("\nStart: %s", req.StartAt.UTC().Format(time.RFC3339))
	}
	return message
}
