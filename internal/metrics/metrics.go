// Copyright 2024 Ella Networks

package metrics

import (
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	smfStats "github.com/ellanetworks/core/internal/smf/stats"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	UpfXdpAborted  prometheus.CounterFunc
	PduSessions    prometheus.CounterFunc
	UpfXdpDrop     prometheus.CounterFunc
	UpfXdpPass     prometheus.CounterFunc
	UpfXdpTx       prometheus.CounterFunc
	UpfXdpRedirect prometheus.CounterFunc

	UpfRx = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "app_n3_rx",
		Help: "The total number of received packets (n3)",
	}, []string{"packet_type"})

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

	// Used for getting difference between two counters to increment the prometheus counter (counters cannot be written only incremented)
	var prevUpfN3Counters ebpf.UpfN3Counters
	go func() {
		time.Sleep(2 * time.Second)
		RxN3PacketCounters := stats.GetUpfN3ExtStatField()
		UpfRx.WithLabelValues("Arp").Add(float64(RxN3PacketCounters.RxArp - prevUpfN3Counters.RxArp))
		UpfRx.WithLabelValues("Icmp").Add(float64(RxN3PacketCounters.RxIcmp - prevUpfN3Counters.RxIcmp))
		UpfRx.WithLabelValues("Icmp6").Add(float64(RxN3PacketCounters.RxIcmp6 - prevUpfN3Counters.RxIcmp6))
		UpfRx.WithLabelValues("Ip4").Add(float64(RxN3PacketCounters.RxIp4 - prevUpfN3Counters.RxIp4))
		UpfRx.WithLabelValues("Ip6").Add(float64(RxN3PacketCounters.RxIp6 - prevUpfN3Counters.RxIp6))
		UpfRx.WithLabelValues("Tcp").Add(float64(RxN3PacketCounters.RxTcp - prevUpfN3Counters.RxTcp))
		UpfRx.WithLabelValues("Udp").Add(float64(RxN3PacketCounters.RxUdp - prevUpfN3Counters.RxUdp))
		UpfRx.WithLabelValues("Other").Add(float64(RxN3PacketCounters.RxOther - prevUpfN3Counters.RxOther))
		UpfRx.WithLabelValues("GtpEcho").Add(float64(RxN3PacketCounters.RxGtpEcho - prevUpfN3Counters.RxGtpEcho))
		UpfRx.WithLabelValues("GtpPdu").Add(float64(RxN3PacketCounters.RxGtpPdu - prevUpfN3Counters.RxGtpPdu))
		UpfRx.WithLabelValues("GtpOther").Add(float64(RxN3PacketCounters.RxGtpOther - prevUpfN3Counters.RxGtpOther))
		UpfRx.WithLabelValues("GtpUnexp").Add(float64(RxN3PacketCounters.RxGtpUnexp - prevUpfN3Counters.RxGtpUnexp))

		prevUpfN3Counters = RxN3PacketCounters
	}()
}
