package raft

import (
	"time"

	gometrics "github.com/hashicorp/go-metrics"
	gometricsprom "github.com/hashicorp/go-metrics/prometheus"
	"github.com/hashicorp/raft"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	peersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ella_raft_peers_total",
		Help: "Total number of peers in the Raft cluster (including self).",
	})
	votersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ella_raft_voters_total",
		Help: "Total number of voters in the Raft cluster.",
	})
	leadershipTransitionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ella_raft_leadership_transitions_total",
		Help: "Number of leadership transitions observed.",
	})
	changesetBytesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_raft_changeset_bytes_total",
		Help: "Total number of SQLite changeset bytes applied through the Raft FSM.",
	})
	changesetApplyDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "app_raft_changeset_apply_duration_seconds",
		Help:    "Latency of SQLite changeset application in the Raft FSM.",
		Buckets: prometheus.DefBuckets,
	})
)

// RegisterMetrics wires the hashicorp/go-metrics Prometheus sink as the
// global metrics sink so all metrics emitted by hashicorp/raft flow into
// Prometheus automatically. It also registers the custom ella_raft_* gauges.
// Call once at process startup before creating the Raft instance.
func RegisterMetrics() {
	sink, err := gometricsprom.NewPrometheusSinkFrom(gometricsprom.PrometheusOpts{
		Expiration: 0, // never expire raft metrics
	})
	if err == nil {
		conf := gometrics.DefaultConfig("")
		conf.EnableHostname = false
		conf.EnableHostnameLabel = false
		conf.EnableRuntimeMetrics = false
		_, _ = gometrics.NewGlobal(conf, sink)
	}

	prometheus.MustRegister(peersTotal)
	prometheus.MustRegister(votersTotal)
	prometheus.MustRegister(leadershipTransitionsTotal)
	prometheus.MustRegister(changesetBytesTotal)
	prometheus.MustRegister(changesetApplyDuration)
}

// IncrLeadershipTransitions bumps the ella_raft_leadership_transitions_total
// counter. Called by the LeaderObserver on each transition.
func IncrLeadershipTransitions() {
	leadershipTransitionsTotal.Inc()
}

// ObserveChangesetApply records the size and latency of an applied changeset.
func ObserveChangesetApply(sizeBytes int, duration time.Duration) {
	changesetBytesTotal.Add(float64(sizeBytes))
	changesetApplyDuration.Observe(duration.Seconds())
}

// runMetricsLoop periodically reads Raft cluster configuration and updates
// the custom ella_raft_* gauges. It runs until stopCh is closed.
func runMetricsLoop(r *raft.Raft, stopCh <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	update := func() {
		future := r.GetConfiguration()
		if err := future.Error(); err != nil {
			return
		}

		servers := future.Configuration().Servers

		var voters int

		for _, s := range servers {
			if s.Suffrage == raft.Voter {
				voters++
			}
		}

		peersTotal.Set(float64(len(servers)))
		votersTotal.Set(float64(voters))
	}

	update()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			update()
		}
	}
}
