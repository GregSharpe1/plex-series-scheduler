package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultGuideLookahead = 7 * 24 * time.Hour

type Loader struct {
	path string
}

type Config struct {
	Plex      PlexConfig      `yaml:"plex"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Rules     []Rule          `yaml:"rules"`
}

type PlexConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type SchedulerConfig struct {
	Interval       Duration `yaml:"interval"`
	DryRun         bool     `yaml:"dryRun"`
	Debug          bool     `yaml:"debug"`
	GuideLookahead Duration `yaml:"guideLookahead"`
	MaxRecordings  int      `yaml:"maxRecordings"`
}

type Rule struct {
	Name              string   `yaml:"name"`
	Enabled           bool     `yaml:"enabled"`
	MatchMode         string   `yaml:"matchMode"`
	Title             string   `yaml:"title"`
	TitleRegex        string   `yaml:"titleRegex"`
	IncludeKeywords   []string `yaml:"includeKeywords"`
	ExcludeKeywords   []string `yaml:"excludeKeywords"`
	Channels          []string `yaml:"channels"`
	PreferFirstMatch  bool     `yaml:"preferFirstMatch"`
	DedupeWindow      Duration `yaml:"dedupeWindow"`
	PaddingBefore     Duration `yaml:"paddingBefore"`
	PaddingAfter      Duration `yaml:"paddingAfter"`
	compiledTitleExpr *regexp.Regexp
}

type Duration struct {
	time.Duration
}

func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

func (l *Loader) Load() (Config, error) {
	return Load(l.path)
}

func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	expanded := os.ExpandEnv(string(content))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Scheduler.Interval.Duration == 0 {
		cfg.Scheduler.Interval = Duration{Duration: 30 * time.Minute}
	}
	if cfg.Scheduler.GuideLookahead.Duration == 0 {
		cfg.Scheduler.GuideLookahead = Duration{Duration: defaultGuideLookahead}
	}
}

func validate(cfg *Config) error {
	if cfg.Plex.URL == "" {
		return fmt.Errorf("plex.url is required")
	}
	if cfg.Plex.Token == "" {
		return fmt.Errorf("plex.token is required")
	}
	if cfg.Scheduler.Interval.Duration <= 0 {
		return fmt.Errorf("scheduler.interval must be greater than zero")
	}
	if cfg.Scheduler.GuideLookahead.Duration <= 0 {
		return fmt.Errorf("scheduler.guideLookahead must be greater than zero")
	}
	if cfg.Scheduler.MaxRecordings < 0 {
		return fmt.Errorf("scheduler.maxRecordings must be zero or greater")
	}

	seenNames := make(map[string]struct{}, len(cfg.Rules))
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("rules[%d]: %w", i, err)
		}
		key := strings.ToLower(rule.Name)
		if _, ok := seenNames[key]; ok {
			return fmt.Errorf("rules[%d]: duplicate rule name %q", i, rule.Name)
		}
		seenNames[key] = struct{}{}
	}

	return nil
}

func validateRule(rule *Rule) error {
	if rule.Name == "" {
		return fmt.Errorf("name is required")
	}
	if rule.MatchMode == "" {
		return fmt.Errorf("matchMode is required")
	}

	switch rule.MatchMode {
	case "exact", "contains":
		if rule.Title == "" {
			return fmt.Errorf("title is required for matchMode %q", rule.MatchMode)
		}
	case "regex":
		if rule.TitleRegex == "" {
			return fmt.Errorf("titleRegex is required for matchMode %q", rule.MatchMode)
		}
		re, err := regexp.Compile(rule.TitleRegex)
		if err != nil {
			return fmt.Errorf("invalid titleRegex: %w", err)
		}
		rule.compiledTitleExpr = re
	case "sports_event":
		if rule.Title == "" && rule.TitleRegex == "" {
			return fmt.Errorf("title or titleRegex is required for matchMode %q", rule.MatchMode)
		}
		if rule.TitleRegex != "" {
			re, err := regexp.Compile(rule.TitleRegex)
			if err != nil {
				return fmt.Errorf("invalid titleRegex: %w", err)
			}
			rule.compiledTitleExpr = re
		}
	default:
		return fmt.Errorf("unsupported matchMode %q", rule.MatchMode)
	}

	if rule.DedupeWindow.Duration < 0 {
		return fmt.Errorf("dedupeWindow must be zero or greater")
	}
	if rule.PaddingBefore.Duration < 0 {
		return fmt.Errorf("paddingBefore must be zero or greater")
	}
	if rule.PaddingAfter.Duration < 0 {
		return fmt.Errorf("paddingAfter must be zero or greater")
	}
	return nil
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value, err)
	}
	d.Duration = duration
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

func (r Rule) CompiledTitleRegex() *regexp.Regexp {
	return r.compiledTitleExpr
}

func (r *Rule) UnmarshalYAML(node *yaml.Node) error {
	type plain Rule
	*r = Rule{Enabled: true}
	return node.Decode((*plain)(r))
}
