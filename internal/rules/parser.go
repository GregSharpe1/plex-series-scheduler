package rules

import "github.com/GregSharpe1/plex-series-scheduler/internal/config"

func Enabled(in []config.Rule) []config.Rule {
	out := make([]config.Rule, 0, len(in))
	for _, rule := range in {
		if rule.Enabled {
			out = append(out, rule)
		}
	}
	return out
}
