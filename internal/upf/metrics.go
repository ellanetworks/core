// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
)

// Eviction reasons for app_upf_dl_buffer_packets_evicted_total. Keep in
// sync with the call sites in buffer_responder.go, the alerting rules in
// observability/grafana/alerting/alerts.yml, and the metrics table in
// docs/reference/observability.md.
const (
	evictedCapHeadDrop = "cap_head_drop"
	evictedTTLExpired  = "ttl_expired"
	evictedByteBudget  = "byte_budget"
	evictedSessionDrop = "session_drop"
	evictedClosed      = "closed"
)

var (
	flowReportsDropped prometheus.Counter

	// The buffer counters are created here rather than in RegisterMetrics
	// so tests can read them without touching the default registry.
	bufferPacketsEvicted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_upf_dl_buffer_packets_evicted_total",
		Help: "Buffered downlink packets discarded, by reason: cap_head_drop (per-queue cap), ttl_expired (paging timed out), byte_budget (global byte budget), session_drop (session deleted or paging failed), closed (responder shut down mid-drain).",
	}, []string{"reason"})

	bufferPacketsReinjected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_upf_dl_buffer_packets_reinjected_total",
		Help: "Buffered downlink packets successfully re-injected.",
	})

	bufferReinjectFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_upf_dl_buffer_reinject_failed_total",
		Help: "Buffered downlink packets whose re-injection failed.",
	})

	bufferRecordsMalformed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_upf_dl_buffer_records_malformed_total",
		Help: "Malformed capture records read from the downlink buffer ring. Non-zero is a bug.",
	})
)

func incCounter(c prometheus.Counter) {
	if c == nil {
		return
	}

	c.Inc()
}

func addCounter(c prometheus.Counter, v float64) {
	if c == nil {
		return
	}

	c.Add(v)
}

func RegisterMetrics() {
	flowReportsDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_flow_reports_dropped_total",
		Help: "Total number of flow reports dropped because the reporter channel was full.",
	})

	prometheus.MustRegister(flowReportsDropped, bufferPacketsEvicted,
		bufferPacketsReinjected, bufferReinjectFailed, bufferRecordsMalformed)

	upfUplinkBytes := prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_uplink_bytes",
		Help: "The total number of uplink bytes going through the data plane (N3 -> N6). This value includes the Ethernet header.",
	}, func() float64 {
		return float64(ebpf.GetN3UplinkThroughputStats(bpfObjects))
	})

	upfDownlinkBytes := prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_downlink_bytes",
		Help: "The total number of downlink bytes going through the data plane (N6 -> N3). This value includes the Ethernet header.",
	}, func() float64 {
		return float64(ebpf.GetN6DownlinkThroughputStats(bpfObjects))
	})

	// Every frame is counted exactly once across the two families:
	// forwarded, by action, or dropped, by reason.
	datapathForwardDesc := prometheus.NewDesc(
		"app_upf_datapath_forward_total",
		"Packets the data plane forwarded, by direction and by the action it took (pass, tx, redirect). The action is the data plane's own decision, not the hook verdict, so it means the same thing in every attach mode.",
		[]string{"direction", "action"},
		nil,
	)

	datapathDropDesc := prometheus.NewDesc(
		"app_upf_datapath_drop_total",
		`Packets the data plane did not forward, by direction and reason. Reasons prefixed internal_ are datapath failures and should stay at zero; reason="unspecified" means a drop site recorded no cause, which is a bug.`,
		[]string{"direction", "reason"},
		nil,
	)

	// The full distribution, including outcomes that are not drops.
	datapathFibLookupDesc := prometheus.NewDesc(
		"app_upf_datapath_fib_lookup_total",
		"FIB lookup outcomes in the data plane.",
		[]string{"direction", "result"},
		nil,
	)

	prometheus.MustRegister(upfUplinkBytes, upfDownlinkBytes)

	// Downlink buffer datapath capture outcomes. The counters live in the
	// BPF program (per-CPU), summed by GetDlBufferCounters.
	dlBufferCaptureDesc := prometheus.NewDesc(
		"app_upf_dl_buffer_capture_total",
		"Downlink packets the datapath captured for buffering, by result. ring_full means the capture ring is too small or the reader too slow; non-zero in normal operation warrants a larger ring.",
		[]string{"result"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		if bpfObjects == nil {
			return
		}

		counters := bpfObjects.GetDlBufferCounters()

		for _, r := range []struct {
			label string
			value uint64
		}{
			{"captured", counters.Captured},
			{"ring_full", counters.RingFull},
			{"too_large", counters.TooLarge},
			{"gso", counters.GSO},
		} {
			ch <- prometheus.MustNewConstMetric(dlBufferCaptureDesc,
				prometheus.CounterValue, float64(r.value), r.label)
		}
	}))

	// Current buffered state, snapshotted from the running responder.
	dlBufferQueuedPacketsDesc := prometheus.NewDesc(
		"app_upf_dl_buffer_queued_packets",
		"Downlink packets currently held in the buffer responder's queues.",
		nil, nil,
	)
	dlBufferQueuedBytesDesc := prometheus.NewDesc(
		"app_upf_dl_buffer_queued_bytes",
		"Bytes of downlink packets currently held in the buffer responder's queues.",
		nil, nil,
	)
	dlBufferSessionsDesc := prometheus.NewDesc(
		"app_upf_dl_buffer_sessions",
		"Sessions with downlink packets currently buffered.",
		nil, nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		responder := activeBufferResponder.Load()
		if responder == nil {
			return
		}

		packets, bytes, sessions := responder.queuedTotals()

		ch <- prometheus.MustNewConstMetric(dlBufferQueuedPacketsDesc, prometheus.GaugeValue, float64(packets))

		ch <- prometheus.MustNewConstMetric(dlBufferQueuedBytesDesc, prometheus.GaugeValue, float64(bytes))

		ch <- prometheus.MustNewConstMetric(dlBufferSessionsDesc, prometheus.GaugeValue, float64(sessions))
	}))

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		for dir, counters := range ebpf.GetDatapathCounters(bpfObjects) {
			for _, a := range []struct {
				label string
				index int
			}{
				{"pass", ebpf.ActionPass},
				{"tx", ebpf.ActionTx},
				{"redirect", ebpf.ActionRedirect},
			} {
				ch <- prometheus.MustNewConstMetric(datapathForwardDesc,
					prometheus.CounterValue, float64(counters.Forwarded[a.index]),
					string(dir), a.label)
			}

			// Publish every reason, including those at zero: an absent
			// series cannot be told apart from one never instrumented.
			for reason, name := range ebpf.DropReasonNames() {
				ch <- prometheus.MustNewConstMetric(datapathDropDesc,
					prometheus.CounterValue, float64(counters.Dropped[reason]),
					string(dir), name)
			}
		}
	}))

	// Register FIB lookup result and ifindex mismatch collector
	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		n3 := ebpf.GetN3RouteStats(bpfObjects)
		n6 := ebpf.GetN6RouteStats(bpfObjects)

		for _, entry := range []struct {
			direction string
			stats     ebpf.RouteStats
		}{
			{"uplink", n3},
			{"downlink", n6},
		} {
			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibSuccess), entry.direction, "success")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibNoNeigh), entry.direction, "no_neigh")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibBlackhole), entry.direction, "blackhole")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibUnreachable), entry.direction, "unreachable")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibProhibit), entry.direction, "prohibit")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibNoSrcAddr), entry.direction, "no_src_addr")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibFragNeeded), entry.direction, "frag_needed")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibNotFwded), entry.direction, "not_fwded")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibFwdDisabled), entry.direction, "fwd_disabled")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibUnsuppLwt), entry.direction, "unsupp_lwt")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibError4), entry.direction, "error_ipv4")

			ch <- prometheus.MustNewConstMetric(datapathFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibError6), entry.direction, "error_ipv6")
		}
	}))

	// Pipeline latency profiling metrics.
	// These are only emitted when the BPF program was compiled with
	// -DENABLE_PROFILING. When profiling is disabled, ProfilingMap is nil,
	// ReadProfilingStats returns nil, and the collector emits nothing — the
	// metrics simply do not appear in the scrape output.
	profilingNsDesc := prometheus.NewDesc(
		"app_upf_pipeline_latency_nanoseconds_total",
		"Total accumulated nanoseconds spent in each pipeline stage. Only present when compiled with -DENABLE_PROFILING.",
		[]string{"direction", "stage"},
		nil,
	)
	profilingCallsDesc := prometheus.NewDesc(
		"app_upf_pipeline_latency_calls_total",
		"Total number of times each pipeline stage was measured. Only present when compiled with -DENABLE_PROFILING.",
		[]string{"direction", "stage"},
		nil,
	)

	type profilingStageInfo struct {
		direction string
		stage     string
	}

	// Index order must match the profile_index enum in profiling.h.
	profilingStages := [ebpf.ProfNumEntries]profilingStageInfo{
		ebpf.ProfN3Total:        {"uplink", "total"},
		ebpf.ProfN6Total:        {"downlink", "total"},
		ebpf.ProfN3PdrLookup:    {"uplink", "pdr_lookup"},
		ebpf.ProfN6PdrLookup:    {"downlink", "pdr_lookup"},
		ebpf.ProfN3MtuCheck:     {"uplink", "mtu_check"},
		ebpf.ProfN6MtuCheck:     {"downlink", "mtu_check"},
		ebpf.ProfN3QerRatelimit: {"uplink", "qer_ratelimit"},
		ebpf.ProfN6QerRatelimit: {"downlink", "qer_ratelimit"},
		ebpf.ProfN3GtpManip:     {"uplink", "gtp_manip"},
		ebpf.ProfN6GtpManip:     {"downlink", "gtp_manip"},
		ebpf.ProfN3SdfFilter:    {"uplink", "sdf_filter"},
		ebpf.ProfN6SdfFilter:    {"downlink", "sdf_filter"},
		ebpf.ProfN3Nat:          {"uplink", "nat"},
		ebpf.ProfN6Nat:          {"downlink", "nat"},
		ebpf.ProfN3FibRouting:   {"uplink", "fib_routing"},
		ebpf.ProfN6FibRouting:   {"downlink", "fib_routing"},
	}

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		stats, err := ebpf.ReadProfilingStats(bpfObjects)
		if err != nil || stats == nil {
			return
		}

		for i, entry := range stats {
			info := profilingStages[i]
			ch <- prometheus.MustNewConstMetric(profilingNsDesc, prometheus.CounterValue, float64(entry.TotalNs), info.direction, info.stage)

			ch <- prometheus.MustNewConstMetric(profilingCallsDesc, prometheus.CounterValue, float64(entry.Count), info.direction, info.stage)
		}
	}))
}
