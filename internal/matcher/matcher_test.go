package matcher

import (
	"testing"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

func TestSportsEventMatching(t *testing.T) {
	engine := New()
	rule := config.Rule{
		Name:            "Formula 1",
		Enabled:         true,
		MatchMode:       "sports_event",
		TitleRegex:      "^Formula 1",
		IncludeKeywords: []string{"Grand Prix", "Qualifying"},
		ExcludeKeywords: []string{"Replay", "Highlights"},
		Channels:        []string{"Sky Sports F1 HD"},
	}

	programme := plex.Programme{
		Title:       "Formula 1: British Grand Prix - Practice 1",
		Description: "Live coverage from Silverstone.",
		ChannelName: "Sky Sports F1 HD",
	}

	if !engine.Match(programme, rule) {
		t.Fatal("expected programme to match sports rule")
	}
}

func TestSportsEventRequiresAnyIncludedKeyword(t *testing.T) {
	engine := New()
	rule := config.Rule{
		Name:            "Formula 1",
		Enabled:         true,
		MatchMode:       "sports_event",
		TitleRegex:      "^Formula 1",
		IncludeKeywords: []string{"Qualifying", "Sprint"},
	}

	programme := plex.Programme{
		Title:       "Formula 1: British Grand Prix - Sprint Race",
		Description: "Silverstone sees the fourth F1 Sprint of the season.",
	}

	if !engine.Match(programme, rule) {
		t.Fatal("expected any matching included keyword to pass")
	}
}

func TestSportsEventRejectsExcludedKeyword(t *testing.T) {
	engine := New()
	rule := config.Rule{
		Name:            "Formula 1",
		Enabled:         true,
		MatchMode:       "sports_event",
		TitleRegex:      "^Formula 1",
		ExcludeKeywords: []string{"Replay"},
	}

	programme := plex.Programme{Title: "Formula 1 Replay"}
	if engine.Match(programme, rule) {
		t.Fatal("expected replay to be rejected")
	}
}

func TestChannelMatchUsesSubstring(t *testing.T) {
	engine := New()
	rule := config.Rule{
		Name:      "Formula 1",
		Enabled:   true,
		MatchMode: "contains",
		Title:     "Formula 1",
		Channels:  []string{"Sky Sports F1"},
	}

	programme := plex.Programme{
		Title:       "Formula 1: British Grand Prix",
		ChannelName: "507 SSPOF1H (Sky Sports F1 HD)",
	}

	if !engine.Match(programme, rule) {
		t.Fatal("expected partial channel name to match")
	}
}
