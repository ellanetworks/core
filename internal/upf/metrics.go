// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
)

var flowReportsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "app_upf_flow_reports_dropped_total",
	Help: "Total number of flow reports dropped before they reached the SMF, by reason.",
}, []string{"reason"})

var dlBufferEvicted = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "app_upf_dl_buffer_evictions_total",
	Help: "Buffered downlink packets discarded before re-injection, by the limit that discarded them.",
}, []string{"reason"})

const (
	flowReportDropChannelFull = "channel_full"

	dlBufferEvictTTL            = "ttl"
	dlBufferEvictByteBudget     = "byte_budget"
	dlBufferEvictQueueDepth     = "queue_depth"
	dlBufferEvictSessionDrop    = "session_drop"
	dlBufferEvictMalformed      = "malformed"
	dlBufferEvictReinjectFailed = "reinject_failed"
)

func RegisterMetrics() {
	prometheus.MustRegister(flowReportsDropped, dlBufferEvicted)

	flowReportsDropped.WithLabelValues(flowReportDropChannelFull)

	for _, reason := range []string{
		dlBufferEvictTTL,
		dlBufferEvictByteBudget,
		dlBufferEvictQueueDepth,
		dlBufferEvictSessionDrop,
		dlBufferEvictMalformed,
		dlBufferEvictReinjectFailed,
	} {
		dlBufferEvicted.WithLabelValues(reason)
	}

	upfBytesDesc := prometheus.NewDesc(
		"app_upf_bytes_total",
		"The total number of bytes going through the data plane, by direction (uplink is N3 -> N6, downlink is N6 -> N3). This value includes the Ethernet header.",
		[]string{"direction"},
		nil,
	)

	dlBufferCaptureDesc := prometheus.NewDesc(
		"app_upf_dl_buffer_capture_attempts_total",
		"Downlink packets for an idle UE the data plane offered to the buffer, by outcome: captured, or the reason the capture was refused.",
		[]string{"result"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(upfBytesDesc, prometheus.CounterValue,
			float64(ebpf.GetN3UplinkThroughputStats(bpfObjects)), "uplink")

		ch <- prometheus.MustNewConstMetric(upfBytesDesc, prometheus.CounterValue,
			float64(ebpf.GetN6DownlinkThroughputStats(bpfObjects)), "downlink")
	}))

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		c := bpfObjects.GetDlBufferCounters()

		for _, entry := range []struct {
			result string
			value  uint64
		}{
			{"captured", c.Captured},
			{"ring_full", c.RingFull},
			{"too_large", c.TooLarge},
			{"gso", c.GSO},
		} {
			ch <- prometheus.MustNewConstMetric(dlBufferCaptureDesc, prometheus.CounterValue,
				float64(entry.value), entry.result)
		}
	}))

	mapEntriesDesc := prometheus.NewDesc(
		"app_upf_bpf_map_entries",
		"Entries currently installed in a data plane BPF map. Divide by app_upf_bpf_map_max_entries for a fill ratio; a value above the maximum means the occupancy bookkeeping has drifted.",
		[]string{"map"},
		nil,
	)

	mapMaxEntriesDesc := prometheus.NewDesc(
		"app_upf_bpf_map_max_entries",
		"Capacity of a data plane BPF map, as declared by its max_entries attribute.",
		[]string{"map"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		for name, usage := range bpfObjects.MapUsage() {
			ch <- prometheus.MustNewConstMetric(mapEntriesDesc, prometheus.GaugeValue,
				float64(usage.Entries), name)

			ch <- prometheus.MustNewConstMetric(mapMaxEntriesDesc, prometheus.GaugeValue,
				float64(usage.MaxEntries), name)
		}
	}))

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

	natEvictionsDesc := prometheus.NewDesc(
		"app_upf_nat_evictions_total",
		"Conntrack entries the data plane found evicted under load and re-created, by the direction of the packet that repaired the pair.",
		[]string{"direction"},
		nil,
	)

	ringbufLostDesc := prometheus.NewDesc(
		"app_upf_ringbuf_events_lost_total",
		"Events the data plane raised but could not place in a ring buffer, by ring buffer name.",
		[]string{"map"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		lost, err := ebpf.RingbufLost(bpfObjects)
		if err != nil {
			ch <- prometheus.NewInvalidMetric(ringbufLostDesc, err)
			return
		}

		for name, count := range lost {
			ch <- prometheus.MustNewConstMetric(ringbufLostDesc, prometheus.CounterValue,
				float64(count), name)
		}
	}))

	// The full distribution, including outcomes that are not drops.
	datapathFibLookupDesc := prometheus.NewDesc(
		"app_upf_datapath_fib_lookup_total",
		"FIB lookup outcomes in the data plane.",
		[]string{"direction", "result"},
		nil,
	)

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

			ch <- prometheus.MustNewConstMetric(natEvictionsDesc,
				prometheus.CounterValue, float64(counters.NatEvictions), string(dir))
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
