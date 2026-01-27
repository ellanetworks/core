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
}
