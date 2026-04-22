package raft

import (
	armonmetrics "github.com/armon/go-metrics"
	armonprom "github.com/armon/go-metrics/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

var changesetBytesTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "app_raft_changeset_bytes_total",
	Help: "Total number of SQLite changeset bytes applied through the Raft FSM.",
})

// RegisterMetrics wires an armon/go-metrics Prometheus sink as the global
// metrics sink. hashicorp/raft (and raft-boltdb) emit metrics through the
// go-metrics/compat shim, which by default routes to armon/go-metrics — so
// this sink receives the raft_* and raft_boltdb_* metrics automatically.
// It also registers the application-specific app_raft_* counter. Call once
// at process startup before creating the Raft instance.
func RegisterMetrics() {
	sink, err := armonprom.NewPrometheusSinkFrom(armonprom.PrometheusOpts{
		Expiration: 0, // never expire raft metrics
	})
	if err == nil {
		conf := armonmetrics.DefaultConfig("")
		conf.EnableHostname = false
		conf.EnableHostnameLabel = false
		conf.EnableRuntimeMetrics = false
		_, _ = armonmetrics.NewGlobal(conf, sink)
	}

	prometheus.MustRegister(changesetBytesTotal)
}

// ObserveChangesetBytes records the size of an applied changeset.
func ObserveChangesetBytes(sizeBytes int) {
	changesetBytesTotal.Add(float64(sizeBytes))
}
