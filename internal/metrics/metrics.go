package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	registry             *prometheus.Registry
	SchedulerRunsTotal   prometheus.Counter
	SchedulerRunDuration prometheus.Histogram
	PlexAPIRequestsTotal *prometheus.CounterVec
	PlexAPIFailuresTotal *prometheus.CounterVec
	RecordingsCreated    prometheus.Counter
	RecordingsSkipped    prometheus.Counter
	DuplicateRecordings  prometheus.Counter
	GuideProgrammes      prometheus.Gauge
	RulesLoaded          prometheus.Gauge
}

func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()
	r := &Registry{
		registry: reg,
		SchedulerRunsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scheduler_runs_total",
			Help: "Total number of scheduler runs.",
		}),
		SchedulerRunDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "scheduler_run_duration_seconds",
			Help:    "Duration of scheduler runs.",
			Buckets: prometheus.DefBuckets,
		}),
		PlexAPIRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "plex_api_requests_total",
			Help: "Total Plex API requests.",
		}, []string{"operation"}),
		PlexAPIFailuresTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "plex_api_failures_total",
			Help: "Total Plex API request failures.",
		}, []string{"operation"}),
		RecordingsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "recordings_created_total",
			Help: "Total recordings created by the scheduler.",
		}),
		RecordingsSkipped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "recordings_skipped_total",
			Help: "Total recordings skipped by the scheduler.",
		}),
		DuplicateRecordings: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "duplicate_recordings_total",
			Help: "Total duplicate recordings prevented by the scheduler.",
		}),
		GuideProgrammes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "guide_programmes_processed",
			Help: "Number of guide programmes processed in the last run.",
		}),
		RulesLoaded: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rules_loaded",
			Help: "Number of rules loaded in the last run.",
		}),
	}

	reg.MustRegister(
		r.SchedulerRunsTotal,
		r.SchedulerRunDuration,
		r.PlexAPIRequestsTotal,
		r.PlexAPIFailuresTotal,
		r.RecordingsCreated,
		r.RecordingsSkipped,
		r.DuplicateRecordings,
		r.GuideProgrammes,
		r.RulesLoaded,
	)

	return r
}

func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}

func (r *Registry) ObserveRun(duration time.Duration) {
	r.SchedulerRunsTotal.Inc()
	r.SchedulerRunDuration.Observe(duration.Seconds())
}
