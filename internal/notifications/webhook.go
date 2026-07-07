package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

type Notifier interface {
	RecordingSubmitted(ctx context.Context, req plex.RecordingRequest) error
}

type nopNotifier struct{}

type webhookNotifier struct {
	url     string
	headers map[string]string
	client  *http.Client
}

type recordingSubmittedPayload struct {
	Event         string    `json:"event"`
	RuleName      string    `json:"ruleName"`
	Title         string    `json:"title"`
	Subtitle      string    `json:"subtitle,omitempty"`
	EpisodeTitle  string    `json:"episodeTitle,omitempty"`
	ChannelName   string    `json:"channelName,omitempty"`
	AiringID      string    `json:"airingId,omitempty"`
	ProgrammeID   string    `json:"programmeId,omitempty"`
	StartAt       time.Time `json:"startAt"`
	EndAt         time.Time `json:"endAt"`
	PaddingBefore string    `json:"paddingBefore,omitempty"`
	PaddingAfter  string    `json:"paddingAfter,omitempty"`
	SubmittedAt   time.Time `json:"submittedAt"`
}

func New(cfg config.NotificationsConfig) (Notifier, error) {
	var notifiers []Notifier
	if cfg.Webhook.URL != "" {
		timeout := cfg.Webhook.Timeout.Duration
		if timeout == 0 {
			timeout = 10 * time.Second
		}

		notifiers = append(notifiers, &webhookNotifier{
			url:     cfg.Webhook.URL,
			headers: cfg.Webhook.Headers,
			client: &http.Client{
				Timeout: timeout,
			},
		})
	}
	if cfg.Pushover.Token != "" && cfg.Pushover.UserKey != "" {
		notifiers = append(notifiers, newPushoverNotifier(cfg.Pushover, defaultPushoverAPIURL))
	}
	if len(notifiers) == 0 {
		return nopNotifier{}, nil
	}
	if len(notifiers) == 1 {
		return notifiers[0], nil
	}
	return multiNotifier(notifiers), nil
}

func (nopNotifier) RecordingSubmitted(context.Context, plex.RecordingRequest) error {
	return nil
}

func (n *webhookNotifier) RecordingSubmitted(ctx context.Context, req plex.RecordingRequest) error {
	payload := recordingSubmittedPayload{
		Event:         "recording_submitted",
		RuleName:      req.RuleName,
		Title:         req.Title,
		Subtitle:      req.Subtitle,
		EpisodeTitle:  req.EpisodeTitle,
		ChannelName:   req.ChannelName,
		AiringID:      req.AiringID,
		ProgrammeID:   req.ProgrammeID,
		StartAt:       req.StartAt.UTC(),
		EndAt:         req.EndAt.UTC(),
		PaddingBefore: req.PaddingBefore.String(),
		PaddingAfter:  req.PaddingAfter.String(),
		SubmittedAt:   time.Now().UTC(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build notification request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range n.headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("notification returned status %s", resp.Status)
	}

	return nil
}
