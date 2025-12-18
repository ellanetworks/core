// Copyright 2024 Ella Networks

package metrics

import (
	"context"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	smfStats "github.com/ellanetworks/core/internal/smf/stats"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	PduSessions  prometheus.CounterFunc
	UpfXdpAction prometheus.CollectorFunc

	UpfUplinkBytes   prometheus.CounterFunc
	UpfDownlinkBytes prometheus.CounterFunc

	// Database metrics
	DatabaseStorageUsed  prometheus.GaugeFunc
	IPAddressesTotal     prometheus.GaugeFunc
	IPAddressesAllocated prometheus.GaugeFunc
)

func RegisterDatabaseMetrics(db *db.Database) {
	DatabaseStorageUsed = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_database_storage_bytes",
		Help: "The total storage used by the database in bytes. This is the size of the database file on disk.",
	}, func() float64 {
		dbSize, err := db.GetSize()
		if err != nil {
			logger.MetricsLog.Warn("Failed to get database storage used", zap.Error(err))
			return 0
		}
		return float64(dbSize)
	})

	IPAddressesTotal = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_ip_addresses_total",
		Help: "The total number of IP addresses available for subscribers",
	}, func() float64 {
		total, err := db.GetIPAddressesTotal()
		if err != nil {
			logger.MetricsLog.Warn("Failed to get total IP addresses", zap.Error(err))
			return 0
		}
		return float64(total)
	})

	IPAddressesAllocated = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_ip_addresses_allocated_total",
		Help: "The total number of IP addresses currently allocated to subscribers",
	}, func() float64 {
		allocated, err := db.GetIPAddressesAllocated(context.Background())
		if err != nil {
			logger.MetricsLog.Warn("Failed to get allocated IP addresses", zap.Error(err))
			return 0
		}
		return float64(allocated)
	})

	prometheus.MustRegister(DatabaseStorageUsed)
	prometheus.MustRegister(IPAddressesTotal)
	prometheus.MustRegister(IPAddressesAllocated)
}

func RegisterSmfMetrics() {
	PduSessions = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_pdu_sessions_total",
		Help: "Number of PDU sessions currently in Ella",
	}, func() float64 {
		return float64(smfStats.GetPDUSessionCount())
	})

	prometheus.MustRegister(PduSessions)
}

func RegisterUPFMetrics(stats *ebpf.UpfXdpActionStatistic) {
	UpfUplinkBytes = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_uplink_bytes",
		Help: "The total number of uplink bytes going through the data plane (N3 -> N6). This value includes the Ethernet header.",
	}, func() float64 {
		uplinkBytes := stats.GetN3UplinkThroughputStats()
		return float64(uplinkBytes)
	})

	UpfDownlinkBytes = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_downlink_bytes",
		Help: "The total number of downlink bytes going through the data plane (N6 -> N3). This value includes the Ethernet header.",
	}, func() float64 {
		downlinkBytes := stats.GetN6DownlinkThroughputStats()
		return float64(downlinkBytes)
	})

	// Metrics for the app_xdp_statistic (xdp_action)
	xdpActionDesc := prometheus.NewDesc(
		"app_xdp_action_total",
		"XDP action per packet.",
		[]string{"interface", "action"},
		nil,
	)

	UpfXdpAction = prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN3Pass()),
			"n3",
			"XDP_PASS",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN3Drop()),
			"n3",
			"XDP_DROP",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN3Tx()),
			"n3",
			"XDP_TX",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN3Aborted()),
			"n3",
			"XDP_ABORTED",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN3Redirect()),
			"n3",
			"XDP_REDIRECT",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN6Pass()),
			"n6",
			"XDP_PASS",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN6Drop()),
			"n6",
			"XDP_DROP",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN6Tx()),
			"n6",
			"XDP_TX",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN6Aborted()),
			"n6",
			"XDP_ABORTED",
		)

		ch <- prometheus.MustNewConstMetric(
			xdpActionDesc,
			prometheus.CounterValue,
			float64(stats.GetN6Redirect()),
			"n6",
			"XDP_REDIRECT",
		)
	})

	// Register metrics
	prometheus.MustRegister(UpfUplinkBytes)
	prometheus.MustRegister(UpfDownlinkBytes)
	prometheus.MustRegister(UpfXdpAction)
}
