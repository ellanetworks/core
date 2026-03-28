// Copyright 2026 Ella Networks

package upf

import (
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterMetrics() {
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

	// Register FIB lookup result collector
	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibSuccess(bpfObjects)), "n3", "success")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibNoNeigh(bpfObjects)), "n3", "no_neigh")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibBlackhole(bpfObjects)), "n3", "blackhole")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibUnreachable(bpfObjects)), "n3", "unreachable")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibProhibit(bpfObjects)), "n3", "prohibit")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibNoSrcAddr(bpfObjects)), "n3", "no_src_addr")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibFragNeeded(bpfObjects)), "n3", "frag_needed")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibNotFwded(bpfObjects)), "n3", "not_fwded")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibFwdDisabled(bpfObjects)), "n3", "fwd_disabled")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN3FibUnsuppLwt(bpfObjects)), "n3", "unsupp_lwt")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibSuccess(bpfObjects)), "n6", "success")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibNoNeigh(bpfObjects)), "n6", "no_neigh")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibBlackhole(bpfObjects)), "n6", "blackhole")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibUnreachable(bpfObjects)), "n6", "unreachable")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibProhibit(bpfObjects)), "n6", "prohibit")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibNoSrcAddr(bpfObjects)), "n6", "no_src_addr")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibFragNeeded(bpfObjects)), "n6", "frag_needed")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibNotFwded(bpfObjects)), "n6", "not_fwded")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibFwdDisabled(bpfObjects)), "n6", "fwd_disabled")

		ch <- prometheus.MustNewConstMetric(xdpFibLookupDesc, prometheus.CounterValue, float64(ebpf.GetN6FibUnsuppLwt(bpfObjects)), "n6", "unsupp_lwt")
	}))

	// Register ifindex mismatch collector
	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(xdpIfindexMismatchDesc, prometheus.CounterValue, float64(ebpf.GetN3IfindexMismatch(bpfObjects)), "n3")

		ch <- prometheus.MustNewConstMetric(xdpIfindexMismatchDesc, prometheus.CounterValue, float64(ebpf.GetN6IfindexMismatch(bpfObjects)), "n6")
	}))
}
