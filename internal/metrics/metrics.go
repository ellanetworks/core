// Copyright 2024 Ella Networks

package metrics

import (
	"context"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	smfStats "github.com/ellanetworks/core/internal/smf/stats"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	PduSessions      prometheus.CounterFunc
	UpfN3XdpAborted  prometheus.CounterFunc
	UpfN3XdpDrop     prometheus.CounterFunc
	UpfN3XdpPass     prometheus.CounterFunc
	UpfN3XdpTx       prometheus.CounterFunc
	UpfN3XdpRedirect prometheus.CounterFunc
	UpfN6XdpAborted  prometheus.CounterFunc
	UpfN6XdpDrop     prometheus.CounterFunc
	UpfN6XdpPass     prometheus.CounterFunc
	UpfN6XdpTx       prometheus.CounterFunc
	UpfN6XdpRedirect prometheus.CounterFunc

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
		Help: "The total number of IP addresses allocated to subscribers",
	}, func() float64 {
		total, err := db.GetIPAddressesTotal()
		if err != nil {
			logger.MetricsLog.Warn("Failed to get total IP addresses", zap.Error(err))
			return 0
		}
		return float64(total)
	})

	IPAddressesAllocated = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_ip_addresses_allocated",
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
	PduSessions = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_pdu_sessions",
		Help: "Number of PDU sessions currently in Ella",
	}, func() float64 {
		return float64(smfStats.GetPDUSessionCount())
	})

	prometheus.MustRegister(PduSessions)
}

type UPFMetrics struct {
	bpfObjects atomic.Pointer[ebpf.BpfObjects]
	registered atomic.Bool
}

func (m *UPFMetrics) get() *ebpf.BpfObjects { return m.bpfObjects.Load() }

func (m *UPFMetrics) SetBpfObjects(bpfObjects *ebpf.BpfObjects) {
	m.bpfObjects.Store(bpfObjects)
}

func (m *UPFMetrics) RegisterUPFMetrics(bpfObjects *ebpf.BpfObjects) {
	// Always set the current source first.
	m.SetBpfObjects(bpfObjects)

	// Already registered? Nothing else to do.
	if m.registered.Load() {
		return
	}

	// Helper to safely read counters from the current objects.
	val := func(f func(*ebpf.BpfObjects) uint64) float64 {
		if o := m.get(); o != nil {
			return float64(f(o))
		}
		return 0
	}

	// Metrics for the app_xdp_statistic (xdp_action)
	UpfN3XdpAborted = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_aborted",
		Help: "The total number of aborted packets (n3)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3Aborted)
	})

	UpfN3XdpDrop = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_drop",
		Help: "The total number of dropped packets (n3)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3Drop)
	})

	UpfN3XdpPass = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_pass",
		Help: "The total number of passed packets (n3)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3Pass)
	})

	UpfN3XdpTx = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_tx",
		Help: "The total number of transmitted packets (n3)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3Tx)
	})

	UpfN3XdpRedirect = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n3_xdp_redirect",
		Help: "The total number of redirected packets (n3)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3Redirect)
	})

	UpfN6XdpAborted = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n6_xdp_aborted",
		Help: "The total number of aborted packets (n6)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN6Aborted)
	})

	UpfN6XdpDrop = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n6_xdp_drop",
		Help: "The total number of dropped packets (n6)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN6Drop)
	})

	UpfN6XdpPass = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n6_xdp_pass",
		Help: "The total number of passed packets (n6)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3Pass)
	})

	UpfN6XdpTx = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n6_xdp_tx",
		Help: "The total number of transmitted packets (n6)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN6Tx)
	})

	UpfN6XdpRedirect = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_n6_xdp_redirect",
		Help: "The total number of redirected packets (n6)",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN6Redirect)
	})

	UpfUplinkBytes = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_uplink_bytes",
		Help: "The total number of uplink bytes going through the data plane (N3 -> N6). This value includes the Ethernet header.",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN3UplinkThroughputStats)
	})

	UpfDownlinkBytes = prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_downlink_bytes",
		Help: "The total number of downlink bytes going through the data plane (N6 -> N3). This value includes the Ethernet header.",
	}, func() float64 {
		return val((*ebpf.BpfObjects).GetN6DownlinkThroughputStats)
	})

	// Register metrics once
	prometheus.MustRegister(
		UpfN3XdpAborted,
		UpfN3XdpDrop,
		UpfN3XdpPass,
		UpfN3XdpTx,
		UpfN3XdpRedirect,
		UpfN6XdpAborted,
		UpfN6XdpDrop,
		UpfN6XdpPass,
		UpfN6XdpTx,
		UpfN6XdpRedirect,
		UpfUplinkBytes,
		UpfDownlinkBytes,
	)

	m.registered.Store(true)
}
