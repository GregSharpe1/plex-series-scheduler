package scheduler

import (
	"testing"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/matcher"
	"github.com/GregSharpe1/plex-series-scheduler/internal/metrics"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

func TestSelectCandidatesPrefersHigherQualityChannel(t *testing.T) {
	rule := config.Rule{
		MatchMode:        "sports_event",
		Channels:         []string{"Sky Sports F1 HD", "Sky Sports F1"},
		PreferFirstMatch: true,
		DedupeWindow:     config.Duration{Duration: 72 * time.Hour},
	}

	base := time.Date(2026, 7, 5, 13, 30, 0, 0, time.UTC)
	candidates := []plex.Programme{
		{Title: "Formula 1", Subtitle: "British Grand Prix - Practice 1", ChannelName: "Sky Sports F1", StartAt: base},
		{Title: "Formula 1", Subtitle: "British Grand Prix - Practice 1", ChannelName: "Sky Sports F1 HD", StartAt: base.Add(5 * time.Minute)},
	}

	selected := selectCandidates(rule, candidates, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))
	if len(selected) != 1 {
		t.Fatalf("selected count = %d, want 1", len(selected))
	}
	if got, want := selected[0].ChannelName, "Sky Sports F1 HD"; got != want {
		t.Fatalf("selected channel = %q, want %q", got, want)
	}
}

func TestSelectCandidatesSeparatesEventsOutsideWindow(t *testing.T) {
	rule := config.Rule{
		MatchMode:    "sports_event",
		DedupeWindow: config.Duration{Duration: 24 * time.Hour},
	}

	base := time.Date(2026, 7, 5, 13, 30, 0, 0, time.UTC)
	candidates := []plex.Programme{
		{Title: "Formula 1", Subtitle: "British Grand Prix - Practice 1", StartAt: base},
		{Title: "Formula 1", Subtitle: "British Grand Prix - Practice 1", StartAt: base.Add(2 * time.Hour)},
		{Title: "Formula 1", Subtitle: "British Grand Prix - Practice 1", StartAt: base.Add(48 * time.Hour)},
	}

	selected := selectCandidates(rule, candidates, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))
	if got, want := len(selected), 2; got != want {
		t.Fatalf("selected count = %d, want %d", got, want)
	}
}

func TestPlanSkipsPastAirings(t *testing.T) {
	now := time.Date(2026, 7, 5, 19, 53, 46, 0, time.UTC)
	s := &Scheduler{
		metrics: metrics.NewRegistry(),
		match:   matcher.New(),
		now: func() time.Time {
			return now
		},
	}

	rule := config.Rule{
		Name:      "Formula 1",
		Enabled:   true,
		MatchMode: "contains",
		Title:     "Formula 1",
	}

	guide := []plex.Programme{
		{Title: "Formula 1", ChannelName: "Sky Sports F1 HD", StartAt: now.Add(-30 * time.Minute)},
		{Title: "Formula 1", ChannelName: "Sky Sports F1 HD", StartAt: now.Add(30 * time.Minute)},
	}

	plan := s.plan(config.Config{}, []config.Rule{rule}, guide, nil)
	if got, want := len(plan.Requests), 1; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
	if !plan.Requests[0].StartAt.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("selected start time = %v, want %v", plan.Requests[0].StartAt, now.Add(30*time.Minute))
	}
	if got, want := plan.Skipped, 0; got != want {
		t.Fatalf("skipped count = %d, want %d", got, want)
	}
}

func TestSelectCandidatesCollapsesRepeatSportsAirings(t *testing.T) {
	rule := config.Rule{
		MatchMode:    "sports_event",
		DedupeWindow: config.Duration{Duration: 72 * time.Hour},
	}

	originallyAvailableAt := time.Date(2026, 7, 5, 13, 0, 0, 0, time.UTC)
	candidates := []plex.Programme{
		{
			Title:                 "Formula 1",
			Subtitle:              "British Grand Prix: Race",
			EpisodeTitle:          "British Grand Prix: Race",
			Description:           "It's lights out at Silverstone for the 2026 British Grand Prix.",
			OriginallyAvailableAt: originallyAvailableAt,
			StartAt:               time.Date(2026, 7, 5, 18, 40, 0, 0, time.UTC),
		},
		{
			Title:                 "Formula 1",
			Subtitle:              "British Grand Prix: Stand Alone Race",
			EpisodeTitle:          "British Grand Prix: Stand Alone Race",
			Description:           "It's lights out at Silverstone for the 2026 British Grand Prix.",
			OriginallyAvailableAt: originallyAvailableAt,
			StartAt:               time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC),
		},
	}

	selected := selectCandidates(rule, candidates, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))
	if got, want := len(selected), 1; got != want {
		t.Fatalf("selected count = %d, want %d", got, want)
	}
	if !selected[0].StartAt.Equal(time.Date(2026, 7, 5, 18, 40, 0, 0, time.UTC)) {
		t.Fatalf("selected start time = %v, want live airing", selected[0].StartAt)
	}
}

func TestSelectCandidatesSkipsMissedFirstSportsAiring(t *testing.T) {
	rule := config.Rule{
		MatchMode:    "sports_event",
		DedupeWindow: config.Duration{Duration: 72 * time.Hour},
	}

	originallyAvailableAt := time.Date(2026, 7, 5, 13, 0, 0, 0, time.UTC)
	candidates := []plex.Programme{
		{
			Title:                 "Formula 1",
			Subtitle:              "British Grand Prix: Race",
			EpisodeTitle:          "British Grand Prix: Race",
			Description:           "It's lights out at Silverstone for the 2026 British Grand Prix.",
			OriginallyAvailableAt: originallyAvailableAt,
			StartAt:               time.Date(2026, 7, 5, 18, 40, 0, 0, time.UTC),
		},
		{
			Title:                 "Formula 1",
			Subtitle:              "British Grand Prix: Stand Alone Race",
			EpisodeTitle:          "British Grand Prix: Stand Alone Race",
			Description:           "It's lights out at Silverstone for the 2026 British Grand Prix.",
			OriginallyAvailableAt: originallyAvailableAt,
			StartAt:               time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC),
		},
	}

	selected := selectCandidates(rule, candidates, time.Date(2026, 7, 5, 20, 0, 0, 0, time.UTC))
	if got, want := len(selected), 0; got != want {
		t.Fatalf("selected count = %d, want %d", got, want)
	}
}

func TestPlanSkipsArchiveSportsRebroadcasts(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.UTC)
	s := &Scheduler{
		metrics: metrics.NewRegistry(),
		match:   matcher.New(),
		now: func() time.Time {
			return now
		},
	}

	rule := config.Rule{
		Name:      "Formula 1",
		Enabled:   true,
		MatchMode: "sports_event",
		Title:     "Formula 1",
	}

	guide := []plex.Programme{
		{
			Title:                 "Formula 1",
			Subtitle:              "Monaco Grand Prix: Race",
			EpisodeTitle:          "Monaco Grand Prix: Race",
			Description:           "Archive Monaco race coverage.",
			OriginallyAvailableAt: time.Date(2021, 5, 24, 13, 0, 0, 0, time.UTC),
			StartAt:               now.Add(48 * time.Hour),
		},
	}

	plan := s.plan(config.Config{}, []config.Rule{rule}, guide, nil)
	if got, want := len(plan.Requests), 0; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
}

func TestPlanAllowsPremiereSportsAiring(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.UTC)
	s := &Scheduler{
		metrics: metrics.NewRegistry(),
		match:   matcher.New(),
		now: func() time.Time {
			return now
		},
	}

	rule := config.Rule{
		Name:      "World Cup",
		Enabled:   true,
		MatchMode: "sports_event",
		Title:     "Live: MOTD FIFA World Cup 2026",
	}

	guide := []plex.Programme{
		{
			Title:                 "Live: MOTD FIFA World Cup 2026",
			Subtitle:              "Round of 16: Mexico v England",
			EpisodeTitle:          "Round of 16: Mexico v England",
			Description:           "Live coverage of Mexico v England.",
			Premiere:              true,
			OriginallyAvailableAt: time.Date(2026, 7, 6, 13, 0, 0, 0, time.UTC),
			StartAt:               now.Add(4 * time.Hour),
		},
	}

	plan := s.plan(config.Config{}, []config.Rule{rule}, guide, nil)
	if got, want := len(plan.Requests), 1; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
}

func TestPlanHonorsGlobalConcurrentMaxRecordings(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.UTC)
	s := &Scheduler{
		metrics: metrics.NewRegistry(),
		match:   matcher.New(),
		now: func() time.Time {
			return now
		},
	}

	rule := config.Rule{
		Name:      "Limited",
		Enabled:   true,
		MatchMode: "contains",
		Title:     "Formula 1",
	}

	guide := []plex.Programme{
		{Title: "Formula 1", ChannelName: "Sky Sports F1 HD", AiringID: "1", StartAt: now.Add(1 * time.Hour), EndAt: now.Add(2 * time.Hour)},
		{Title: "Formula 1", ChannelName: "Sky Sports F1 HD", AiringID: "2", StartAt: now.Add(90 * time.Minute), EndAt: now.Add(150 * time.Minute)},
		{Title: "Formula 1", ChannelName: "Sky Sports F1 HD", AiringID: "3", StartAt: now.Add(3 * time.Hour), EndAt: now.Add(4 * time.Hour)},
	}

	plan := s.plan(config.Config{Scheduler: config.SchedulerConfig{MaxRecordings: 1}}, []config.Rule{rule}, guide, nil)
	if got, want := len(plan.Requests), 2; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
	if got, want := plan.Skipped, 1; got != want {
		t.Fatalf("skipped count = %d, want %d", got, want)
	}
}

func TestPlanHonorsGlobalConcurrentMaxAgainstScheduled(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.UTC)
	s := &Scheduler{
		metrics: metrics.NewRegistry(),
		match:   matcher.New(),
		now: func() time.Time {
			return now
		},
	}

	rule := config.Rule{Name: "Limited", Enabled: true, MatchMode: "contains", Title: "Formula 1"}
	guide := []plex.Programme{{Title: "Formula 1", AiringID: "1", StartAt: now.Add(1 * time.Hour), EndAt: now.Add(2 * time.Hour)}}
	scheduled := []plex.ScheduledRecording{{AiringID: "existing", StartAt: now.Add(90 * time.Minute), EndAt: now.Add(150 * time.Minute)}}

	plan := s.plan(config.Config{Scheduler: config.SchedulerConfig{MaxRecordings: 1}}, []config.Rule{rule}, guide, scheduled)
	if got, want := len(plan.Requests), 0; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
}
