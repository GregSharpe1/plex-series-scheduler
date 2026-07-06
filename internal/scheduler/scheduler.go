package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/matcher"
	"github.com/GregSharpe1/plex-series-scheduler/internal/metrics"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
	"github.com/GregSharpe1/plex-series-scheduler/internal/rules"
)

type ConfigLoader interface {
	Load() (config.Config, error)
}

type ClientFactory func(config.PlexConfig) (plex.Client, error)

type Scheduler struct {
	loader  ConfigLoader
	clients ClientFactory
	metrics *metrics.Registry
	logger  *slog.Logger
	match   *matcher.Engine
	now     func() time.Time
}

type Plan struct {
	Requests []plex.RecordingRequest
	Skipped  int
	Matched  int
}

func New(loader ConfigLoader, clients ClientFactory, registry *metrics.Registry, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		loader:  loader,
		clients: clients,
		metrics: registry,
		logger:  logger,
		match:   matcher.New(),
		now:     time.Now,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	for {
		cfg, err := s.loader.Load()
		if err != nil {
			return err
		}

		if err := s.runWithConfig(ctx, cfg); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.Scheduler.Interval.Duration):
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	cfg, err := s.loader.Load()
	if err != nil {
		return err
	}
	return s.runWithConfig(ctx, cfg)
}

func (s *Scheduler) runWithConfig(ctx context.Context, cfg config.Config) error {
	start := time.Now().UTC()
	activeRules := rules.Enabled(cfg.Rules)
	s.metrics.RulesLoaded.Set(float64(len(activeRules)))

	s.logger.Info("scheduler run started",
		slog.Time("start_time", start),
		slog.Int("rules_loaded", len(activeRules)),
		slog.Bool("dry_run", cfg.Scheduler.DryRun),
	)

	client, err := s.clients(cfg.Plex)
	if err != nil {
		return fmt.Errorf("build plex client: %w", err)
	}

	guide, err := client.Guide(ctx, cfg.Scheduler.GuideLookahead.Duration)
	if err != nil {
		s.metrics.PlexAPIFailuresTotal.WithLabelValues("guide").Inc()
		return fmt.Errorf("query guide: %w", err)
	}
	s.metrics.PlexAPIRequestsTotal.WithLabelValues("guide").Inc()
	guide = normalizeProgrammes(guide)
	s.metrics.GuideProgrammes.Set(float64(len(guide)))

	scheduled, err := client.ScheduledRecordings(ctx)
	if err != nil {
		s.metrics.PlexAPIFailuresTotal.WithLabelValues("scheduled_recordings").Inc()
		return fmt.Errorf("query scheduled recordings: %w", err)
	}
	s.metrics.PlexAPIRequestsTotal.WithLabelValues("scheduled_recordings").Inc()

	plan := s.plan(cfg, activeRules, guide, scheduled)
	createdCount := 0
	skippedCount := plan.Skipped
	for _, req := range plan.Requests {
		if cfg.Scheduler.DryRun {
			s.metrics.RecordingsSkipped.Inc()
			skippedCount++
			attrs := []any{
				slog.String("rule", req.RuleName),
				slog.String("airing_id", req.AiringID),
				slog.String("programme_id", req.ProgrammeID),
			}
			if cfg.Scheduler.Debug {
				attrs = append(attrs,
					slog.String("channel", req.ChannelName),
					slog.Time("start_time", req.StartAt.UTC()),
				)
			}
			s.logger.Info("dry run would schedule recording", attrs...)
			continue
		}

		if err := client.CreateRecording(ctx, req); err != nil {
			s.metrics.PlexAPIFailuresTotal.WithLabelValues("create_recording").Inc()
			return fmt.Errorf("create recording for airing %s: %w", req.AiringID, err)
		}
		s.metrics.PlexAPIRequestsTotal.WithLabelValues("create_recording").Inc()
		s.metrics.RecordingsCreated.Inc()
		createdCount++
	}

	duration := time.Since(start)
	s.metrics.ObserveRun(duration)
	s.logger.Info("scheduler run completed",
		slog.Time("start_time", start),
		slog.Duration("duration", duration),
		slog.Int("programmes_processed", len(guide)),
		slog.Int("rules_loaded", len(activeRules)),
		slog.Int("matches_found", plan.Matched),
		slog.Int("recordings_created", createdCount),
		slog.Int("recordings_skipped", skippedCount),
	)

	return nil
}

func (s *Scheduler) plan(cfg config.Config, activeRules []config.Rule, guide []plex.Programme, scheduled []plex.ScheduledRecording) Plan {
	now := s.now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	existing := make(map[string]struct{}, len(scheduled))
	for _, item := range scheduled {
		existing[exactFingerprint(item.Title, item.Subtitle, item.EpisodeTitle, item.ChannelName, item.StartAt)] = struct{}{}
		if item.AiringID != "" {
			existing[item.AiringID] = struct{}{}
		}
	}

	plan := Plan{}
	for _, rule := range activeRules {
		candidates := make([]plex.Programme, 0)
		for _, programme := range guide {
			if rule.MatchMode == "sports_event" && !programme.Premiere && !programme.OriginallyAvailableAt.IsZero() && programme.OriginallyAvailableAt.Before(todayStart) {
				continue
			}
			if rule.MatchMode != "sports_event" && !programme.StartAt.After(now) {
				continue
			}
			if s.match.Match(programme, rule) {
				plan.Matched++
				candidates = append(candidates, programme)
			}
		}

		selected := selectCandidates(rule, candidates, now)
		for _, programme := range selected {
			if !programme.StartAt.After(now) {
				plan.Skipped++
				continue
			}
			if cfg.Scheduler.MaxRecordings > 0 && concurrentRecordingsAt(programme.StartAt, programme.EndAt, scheduled, plan.Requests) >= cfg.Scheduler.MaxRecordings {
				plan.Skipped++
				continue
			}
			fingerprint := exactFingerprint(programme.Title, programme.Subtitle, programme.EpisodeTitle, programme.ChannelName, programme.StartAt)
			if _, ok := existing[fingerprint]; ok {
				plan.Skipped++
				s.metrics.DuplicateRecordings.Inc()
				continue
			}
			if programme.AiringID != "" {
				if _, ok := existing[programme.AiringID]; ok {
					plan.Skipped++
					s.metrics.DuplicateRecordings.Inc()
					continue
				}
			}

			plan.Requests = append(plan.Requests, plex.RecordingRequest{
				GUID:          programme.GUID,
				RatingKey:     programme.RatingKey,
				AiringID:      programme.AiringID,
				ProgrammeID:   programme.ProgrammeID,
				ChannelID:     programme.ChannelID,
				ChannelName:   programme.ChannelName,
				AiringChannel: programme.AiringChannel,
				StartAt:       programme.StartAt,
				EndAt:         programme.EndAt,
				PaddingBefore: rule.PaddingBefore.Duration,
				PaddingAfter:  rule.PaddingAfter.Duration,
				RuleName:      rule.Name,
			})
			existing[fingerprint] = struct{}{}
			if programme.AiringID != "" {
				existing[programme.AiringID] = struct{}{}
			}
		}
	}

	return plan
}

func concurrentRecordingsAt(startAt, endAt time.Time, scheduled []plex.ScheduledRecording, planned []plex.RecordingRequest) int {
	count := 0
	for _, item := range scheduled {
		if timeRangesOverlap(startAt, endAt, item.StartAt, item.EndAt) {
			count++
		}
	}
	for _, item := range planned {
		if timeRangesOverlap(startAt, endAt, item.StartAt, item.EndAt) {
			count++
		}
	}
	return count
}

func timeRangesOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	if aStart.IsZero() || bStart.IsZero() {
		return false
	}
	if aEnd.IsZero() {
		aEnd = aStart
	}
	if bEnd.IsZero() {
		bEnd = bStart
	}
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

func selectCandidates(rule config.Rule, candidates []plex.Programme, now time.Time) []plex.Programme {
	if len(candidates) == 0 {
		return nil
	}

	if rule.MatchMode != "sports_event" || rule.DedupeWindow.Duration <= 0 {
		sortProgrammes(rule, candidates)
		return candidates
	}

	grouped := make(map[string][]plex.Programme)
	for _, candidate := range candidates {
		key := sameEventKey(candidate)
		grouped[key] = append(grouped[key], candidate)
	}

	selected := make([]plex.Programme, 0, len(grouped))
	for _, group := range grouped {
		sortProgrammes(rule, group)
		for _, cluster := range clusterByWindow(group, rule.DedupeWindow.Duration) {
			if len(cluster) == 0 {
				continue
			}
			if !cluster[0].StartAt.After(now) {
				continue
			}
			selected = append(selected, cluster[0])
		}
	}

	sortProgrammes(rule, selected)
	return selected
}

func sortProgrammes(rule config.Rule, programmes []plex.Programme) {
	channelOrder := make(map[string]int, len(rule.Channels))
	for i, channel := range rule.Channels {
		channelOrder[strings.ToLower(strings.TrimSpace(channel))] = i
	}

	sort.Slice(programmes, func(i, j int) bool {
		leftQuality := channelQuality(programmes[i].ChannelName)
		rightQuality := channelQuality(programmes[j].ChannelName)
		if leftQuality != rightQuality {
			return leftQuality > rightQuality
		}
		if !programmes[i].StartAt.Equal(programmes[j].StartAt) {
			return programmes[i].StartAt.Before(programmes[j].StartAt)
		}
		leftIndex := channelPreferenceIndex(programmes[i].ChannelName, channelOrder)
		rightIndex := channelPreferenceIndex(programmes[j].ChannelName, channelOrder)
		if leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		return programmes[i].AiringID < programmes[j].AiringID
	})
}

func clusterByWindow(programmes []plex.Programme, window time.Duration) [][]plex.Programme {
	if len(programmes) == 0 {
		return nil
	}

	clusters := make([][]plex.Programme, 0, len(programmes))
	current := []plex.Programme{programmes[0]}
	anchor := programmes[0].StartAt

	for _, programme := range programmes[1:] {
		if programme.StartAt.Sub(anchor) <= window {
			current = append(current, programme)
			continue
		}
		clusters = append(clusters, current)
		current = []plex.Programme{programme}
		anchor = programme.StartAt
	}

	return append(clusters, current)
}

func exactFingerprint(title, subtitle, episodeTitle, channel string, startAt time.Time) string {
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(title),
		strings.TrimSpace(subtitle),
		strings.TrimSpace(episodeTitle),
		strings.TrimSpace(channel),
		startAt.UTC().Format(time.RFC3339),
	}, "|"))
}

func sameEventKey(programme plex.Programme) string {
	originallyAvailableAt := ""
	if !programme.OriginallyAvailableAt.IsZero() {
		originallyAvailableAt = programme.OriginallyAvailableAt.UTC().Format("2006-01-02 15:04:05")
	}
	parts := []string{
		normalizeEventField(programme.Title),
		normalizeEventDescriptor(programme),
		normalizeEventField(originallyAvailableAt),
		normalizeEventField(programme.Description),
	}
	return strings.Join(parts, "|")
}

func normalizeEventDescriptor(programme plex.Programme) string {
	descriptor := firstNonEmptyString(programme.Subtitle, programme.EpisodeTitle)
	if cut := strings.LastIndex(descriptor, ":"); cut >= 0 {
		descriptor = descriptor[cut+1:]
	}
	descriptor = normalizeEventField(descriptor)
	replacer := strings.NewReplacer(
		"stand alone race", "race",
		"grand prix sunday", "grand prix sunday",
	)
	return strings.TrimSpace(replacer.Replace(descriptor))
}

func normalizeEventField(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"(replay)", "",
		"replay", "",
		"repeat", "",
		"  ", " ",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeProgrammes(in []plex.Programme) []plex.Programme {
	out := make([]plex.Programme, 0, len(in))
	for _, programme := range in {
		programme.StartAt = programme.StartAt.UTC()
		programme.EndAt = programme.EndAt.UTC()
		out = append(out, programme)
	}
	return out
}

func channelQuality(channel string) int {
	upper := strings.ToUpper(channel)
	switch {
	case strings.Contains(upper, "UHD"), strings.Contains(upper, "4K"), strings.Contains(upper, "FHD"):
		return 3
	case strings.Contains(upper, "HD"):
		return 2
	default:
		return 1
	}
}

func channelPreferenceIndex(channel string, channelOrder map[string]int) int {
	if idx, ok := channelOrder[strings.ToLower(strings.TrimSpace(channel))]; ok {
		return idx
	}
	return len(channelOrder) + 1
}
