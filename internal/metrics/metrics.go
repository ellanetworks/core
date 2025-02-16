// Copyright 2024 Ella Networks

package metrics

import (
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	smfStats "github.com/ellanetworks/core/internal/smf/stats"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	UpfXdpAborted  prometheus.CounterFunc
	PduSessions    prometheus.CounterFunc
	UpfXdpDrop     prometheus.CounterFunc
	UpfXdpPass     prometheus.CounterFunc
	UpfXdpTx       prometheus.CounterFunc
	UpfXdpRedirect prometheus.CounterFunc

	UpfUplinkBytes prometheus.CounterFunc
	// UpfDownlinkBytes prometheus.CounterFunc

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
			logger.MetricsLog.Warnf("Failed to get database storage used: %v", err)
			return 0
		}
		return float64(dbSize)
	})

	IPAddressesTotal = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_ip_addresses_total",
		Help: "The total number of IP addresses allocated to subscribers",
	}, func() float64 {
		total, err := db.GetIPAddressesTotal()
		if err != nil {
			logger.MetricsLog.Warnf("Failed to get total IP addresses: %v", err)
			return 0
		}
		return float64(total)
	})

	IPAddressesAllocated = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_ip_addresses_allocated",
		Help: "The total number of IP addresses currently allocated to subscribers",
	}, func() float64 {
		allocated, err := db.GetIPAddressesAllocated()
		if err != nil {
			logger.MetricsLog.Warnf("Failed to get allocated IP addresses: %v", err)
			return 0
		}
		return float64(allocated)
	})

	prometheus.MustRegister(DatabaseStorageUsed)
	prometheus.MustRegister(IPAddressesTotal)
	prometheus.MustRegister(IPAddressesAllocated)
}

func RegisterSmfMetrics() {
	PduSessions = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_pdu_sessions",
		Help: "Number of PDU sessions currently in Ella",
	}, func() float64 {
		return float64(smfStats.GetPDUSessionCount())
	})

	prometheus.MustRegister(PduSessions)
}

func RegisterUPFMetrics(stats ebpf.UpfXdpActionStatistic, conn *core.PfcpConnection) {
	// Metrics for the app_xdp_statistic (xdp_action)
	UpfXdpAborted = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_aborted",
		Help: "The total number of aborted packets (n3)",
	}, func() float64 {
		return float64(stats.GetN3Aborted())
	})

	UpfXdpDrop = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_drop",
		Help: "The total number of dropped packets (n3)",
	}, func() float64 {
		return float64(stats.GetN3Drop())
	})

	UpfXdpPass = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_pass",
		Help: "The total number of passed packets (n3)",
	}, func() float64 {
		return float64(stats.GetN3Pass())
	})

	UpfXdpTx = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_tx",
		Help: "The total number of transmitted packets (n3)",
	}, func() float64 {
		return float64(stats.GetN3Tx())
	})

	UpfXdpRedirect = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_redirect",
		Help: "The total number of redirected packet (n3)s",
	}, func() float64 {
		return float64(stats.GetN3Redirect())
	})

	UpfUplinkBytes = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_uplink_bytes",
		Help: "The total number of uplink bytes going through the data plane (N3 -> N6). This value includes the Ethernet header.",
	}, func() float64 {
		uplinkBytes := stats.GetN3UplinkThroughputStats()
		return float64(uplinkBytes)
	})

	// UpfDownlinkBytes = prometheus.NewCounterFunc(prometheus.CounterOpts{
	// 	Name: "app_downlink_bytes",
	// 	Help: "The total number of downlink bytes going through the data plane (N6 -> N3). This value includes the Ethernet header.",
	// }, func() float64 {
	// 	_, downlinkBytes := stats.GetThroughputStats()
	// 	return float64(downlinkBytes)
	// })

	// Register metrics
	prometheus.MustRegister(UpfXdpAborted)
	prometheus.MustRegister(UpfXdpDrop)
	prometheus.MustRegister(UpfXdpPass)
	prometheus.MustRegister(UpfXdpTx)
	prometheus.MustRegister(UpfXdpRedirect)
	prometheus.MustRegister(UpfUplinkBytes)
	// prometheus.MustRegister(UpfDownlinkBytes)

}
