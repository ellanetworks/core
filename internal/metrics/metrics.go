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
	PfcpMessageRx = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "app_pfcp_rx",
		Help: "The total number of received PFCP messages",
	}, []string{"message_name"})

	PfcpMessageTx = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "app_pfcp_tx",
		Help: "The total number of transmitted PFCP messages",
	}, []string{"message_name"})

	PfcpMessageRxErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "app_pfcp_rx_errors",
		Help: "The total number of received PFCP messages with cause code",
	}, []string{"message_name", "cause_code"})

	UpfXdpAborted  prometheus.CounterFunc
	PduSessions    prometheus.CounterFunc
	UpfXdpDrop     prometheus.CounterFunc
	UpfXdpPass     prometheus.CounterFunc
	UpfXdpTx       prometheus.CounterFunc
	UpfXdpRedirect prometheus.CounterFunc

	UpfRx = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "app_rx",
		Help: "The total number of received packets",
	}, []string{"packet_type"})

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
		Name: "app_xdp_aborted",
		Help: "The total number of aborted packets",
	}, func() float64 {
		return float64(stats.GetAborted())
	})

	UpfXdpDrop = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_xdp_drop",
		Help: "The total number of dropped packets",
	}, func() float64 {
		return float64(stats.GetDrop())
	})

	UpfXdpPass = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_xdp_pass",
		Help: "The total number of passed packets",
	}, func() float64 {
		return float64(stats.GetPass())
	})

	UpfXdpTx = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_xdp_tx",
		Help: "The total number of transmitted packets",
	}, func() float64 {
		return float64(stats.GetTx())
	})

	UpfXdpRedirect = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_xdp_redirect",
		Help: "The total number of redirected packets",
	}, func() float64 {
		return float64(stats.GetRedirect())
	})

	// Register metrics
	prometheus.MustRegister(UpfXdpAborted)
	prometheus.MustRegister(UpfXdpDrop)
	prometheus.MustRegister(UpfXdpPass)
	prometheus.MustRegister(UpfXdpTx)
	prometheus.MustRegister(UpfXdpRedirect)

	// Used for getting difference between two counters to increment the prometheus counter (counters cannot be written only incremented)
	var prevUpfCounters ebpf.UpfCounters
	go func() {
		time.Sleep(2 * time.Second)
		RxPacketCounters := stats.GetUpfExtStatField()
		UpfRx.WithLabelValues("Arp").Add(float64(RxPacketCounters.RxArp - prevUpfCounters.RxArp))
		UpfRx.WithLabelValues("Icmp").Add(float64(RxPacketCounters.RxIcmp - prevUpfCounters.RxIcmp))
		UpfRx.WithLabelValues("Icmp6").Add(float64(RxPacketCounters.RxIcmp6 - prevUpfCounters.RxIcmp6))
		UpfRx.WithLabelValues("Ip4").Add(float64(RxPacketCounters.RxIp4 - prevUpfCounters.RxIp4))
		UpfRx.WithLabelValues("Ip6").Add(float64(RxPacketCounters.RxIp6 - prevUpfCounters.RxIp6))
		UpfRx.WithLabelValues("Tcp").Add(float64(RxPacketCounters.RxTcp - prevUpfCounters.RxTcp))
		UpfRx.WithLabelValues("Udp").Add(float64(RxPacketCounters.RxUdp - prevUpfCounters.RxUdp))
		UpfRx.WithLabelValues("Other").Add(float64(RxPacketCounters.RxOther - prevUpfCounters.RxOther))
		UpfRx.WithLabelValues("GtpEcho").Add(float64(RxPacketCounters.RxGtpEcho - prevUpfCounters.RxGtpEcho))
		UpfRx.WithLabelValues("GtpPdu").Add(float64(RxPacketCounters.RxGtpPdu - prevUpfCounters.RxGtpPdu))
		UpfRx.WithLabelValues("GtpOther").Add(float64(RxPacketCounters.RxGtpOther - prevUpfCounters.RxGtpOther))
		UpfRx.WithLabelValues("GtpUnexp").Add(float64(RxPacketCounters.RxGtpUnexp - prevUpfCounters.RxGtpUnexp))

		prevUpfCounters = RxPacketCounters
	}()
}
