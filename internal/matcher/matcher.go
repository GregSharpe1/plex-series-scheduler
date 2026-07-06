package matcher

import (
	"regexp"
	"strings"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
)

type Matcher interface {
	Match(programme plex.Programme, rule config.Rule) bool
}

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Match(programme plex.Programme, rule config.Rule) bool {
	if !matchesChannel(programme.ChannelName, rule.Channels) {
		return false
	}
	if !matchesTitle(programme.Title, rule) {
		return false
	}
	if !containsRequiredKeyword(programme, rule.IncludeKeywords) {
		return false
	}
	if containsAnyKeyword(programme, rule.ExcludeKeywords) {
		return false
	}
	return true
}

func matchesChannel(channel string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	for _, candidate := range allowed {
		normalizedCandidate := strings.ToLower(strings.TrimSpace(candidate))
		if normalizedCandidate != "" && strings.Contains(normalizedChannel, normalizedCandidate) {
			return true
		}
	}
	return false
}

func matchesTitle(title string, rule config.Rule) bool {
	switch rule.MatchMode {
	case "exact":
		return strings.EqualFold(title, rule.Title)
	case "contains":
		return strings.Contains(strings.ToLower(title), strings.ToLower(rule.Title))
	case "regex":
		return matchesRegex(title, rule.CompiledTitleRegex(), rule.TitleRegex)
	case "sports_event":
		if rule.CompiledTitleRegex() != nil || rule.TitleRegex != "" {
			return matchesRegex(title, rule.CompiledTitleRegex(), rule.TitleRegex)
		}
		return strings.Contains(strings.ToLower(title), strings.ToLower(rule.Title))
	default:
		return false
	}
}

func matchesRegex(value string, compiled *regexp.Regexp, source string) bool {
	if compiled != nil {
		return compiled.MatchString(value)
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func containsRequiredKeyword(programme plex.Programme, keywords []string) bool {
	if len(keywords) == 0 {
		return true
	}
	blob := programBlob(programme)
	for _, keyword := range keywords {
		if strings.Contains(blob, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func containsAnyKeyword(programme plex.Programme, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	blob := programBlob(programme)
	for _, keyword := range keywords {
		if strings.Contains(blob, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func programBlob(programme plex.Programme) string {
	return strings.ToLower(strings.Join([]string{
		programme.Title,
		programme.Subtitle,
		programme.EpisodeTitle,
		programme.Description,
	}, "\n"))
}
