// Copyright 2026 Ella Networks

package upf

import (
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
)

var flowReportsDropped prometheus.Counter

func RegisterMetrics() {
	flowReportsDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_flow_reports_dropped_total",
		Help: "Total number of flow reports dropped because the reporter channel was full.",
	})

	prometheus.MustRegister(flowReportsDropped)

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

	// XDP action metrics with labels for interface and action
	xdpActionDesc := prometheus.NewDesc(
		"app_xdp_action_total",
		"XDP action per packet.",
		[]string{"interface", "action"},
		nil,
	)

	// FIB lookup result metrics with labels for interface and result
	xdpFibLookupDesc := prometheus.NewDesc(
		"app_xdp_fib_lookup_total",
		"FIB lookup outcomes in the XDP data plane.",
		[]string{"interface", "result"},
		nil,
	)

	// Ifindex mismatch metrics with label for interface
	xdpIfindexMismatchDesc := prometheus.NewDesc(
		"app_xdp_ifindex_mismatch_total",
		"Packets dropped because the FIB-resolved interface did not match the expected N3/N6 interface.",
		[]string{"interface"},
		nil,
	)

	prometheus.MustRegister(upfUplinkBytes, upfDownlinkBytes)

	// Register XDP action collector that produces metrics with labels
	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN3Pass(bpfObjects)), "n3", "XDP_PASS")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN3Drop(bpfObjects)), "n3", "XDP_DROP")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN3Tx(bpfObjects)), "n3", "XDP_TX")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN3Aborted(bpfObjects)), "n3", "XDP_ABORTED")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN3Redirect(bpfObjects)), "n3", "XDP_REDIRECT")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN6Pass(bpfObjects)), "n6", "XDP_PASS")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN6Drop(bpfObjects)), "n6", "XDP_DROP")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN6Tx(bpfObjects)), "n6", "XDP_TX")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN6Aborted(bpfObjects)), "n6", "XDP_ABORTED")

		ch <- prometheus.MustNewConstMetric(xdpActionDesc, prometheus.CounterValue, float64(ebpf.GetN6Redirect(bpfObjects)), "n6", "XDP_REDIRECT")
	}))

	// Register FIB lookup result and ifindex mismatch collector
	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		n3 := ebpf.GetN3RouteStats(bpfObjects)
		n6 := ebpf.GetN6RouteStats(bpfObjects)

		for _, entry := range []struct {
			iface string
			stats ebpf.RouteStats
		}{
			{"n3", n3},
			{"n6", n6},
		} {
			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibSuccess), entry.iface, "success")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibNoNeigh), entry.iface, "no_neigh")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibBlackhole), entry.iface, "blackhole")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibUnreachable), entry.iface, "unreachable")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibProhibit), entry.iface, "prohibit")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibNoSrcAddr), entry.iface, "no_src_addr")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibFragNeeded), entry.iface, "frag_needed")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibNotFwded), entry.iface, "not_fwded")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibFwdDisabled), entry.iface, "fwd_disabled")

			ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(entry.stats.FibUnsuppLwt), entry.iface, "unsupp_lwt")

			ch <- prometheus.MustNewConstMetric(xdpIfindexMismatchDesc, prometheus.CounterValue, float64(entry.stats.IfindexMismatch), entry.iface)
		}
	}))

	// Pipeline latency profiling metrics.
	// These are only emitted when the BPF program was compiled with
	// -DENABLE_PROFILING. When profiling is disabled, ProfilingMap is nil,
	// ReadProfilingStats returns nil, and the collector emits nothing — the
	// metrics simply do not appear in the scrape output.
	profilingNsDesc := prometheus.NewDesc(
		"app_upf_pipeline_latency_nanoseconds_total",
		"Total accumulated nanoseconds spent in each XDP UPF pipeline stage. Only present when compiled with -DENABLE_PROFILING.",
		[]string{"interface", "stage"},
		nil,
	)
	profilingCallsDesc := prometheus.NewDesc(
		"app_upf_pipeline_latency_calls_total",
		"Total number of times each XDP UPF pipeline stage was measured. Only present when compiled with -DENABLE_PROFILING.",
		[]string{"interface", "stage"},
		nil,
	)

	type profilingStageInfo struct {
		iface string
		stage string
	}

	// Index order must match the profile_index enum in profiling.h.
	profilingStages := [ebpf.ProfNumEntries]profilingStageInfo{
		ebpf.ProfN3Total:        {"n3", "total"},
		ebpf.ProfN6Total:        {"n6", "total"},
		ebpf.ProfN3PdrLookup:    {"n3", "pdr_lookup"},
		ebpf.ProfN6PdrLookup:    {"n6", "pdr_lookup"},
		ebpf.ProfN3MtuCheck:     {"n3", "mtu_check"},
		ebpf.ProfN6MtuCheck:     {"n6", "mtu_check"},
		ebpf.ProfN3QerRatelimit: {"n3", "qer_ratelimit"},
		ebpf.ProfN6QerRatelimit: {"n6", "qer_ratelimit"},
		ebpf.ProfN3GtpManip:     {"n3", "gtp_manip"},
		ebpf.ProfN6GtpManip:     {"n6", "gtp_manip"},
		ebpf.ProfN3SdfFilter:    {"n3", "sdf_filter"},
		ebpf.ProfN6SdfFilter:    {"n6", "sdf_filter"},
		ebpf.ProfN3Nat:          {"n3", "nat"},
		ebpf.ProfN6Nat:          {"n6", "nat"},
		ebpf.ProfN3FibRouting:   {"n3", "fib_routing"},
		ebpf.ProfN6FibRouting:   {"n6", "fib_routing"},
		ebpf.ProfN3Noop:         {"n3", "noop"},
		ebpf.ProfN6Noop:         {"n6", "noop"},
	}

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		stats, err := ebpf.ReadProfilingStats(bpfObjects)
		if err != nil || stats == nil {
			return
		}

		for i, entry := range stats {
			info := profilingStages[i]
			ch <- prometheus.MustNewConstMetric(profilingNsDesc, prometheus.CounterValue, float64(entry.TotalNs), info.iface, info.stage)

			ch <- prometheus.MustNewConstMetric(profilingCallsDesc, prometheus.CounterValue, float64(entry.Count), info.iface, info.stage)
		}
	}))
}
