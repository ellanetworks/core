// Copyright 2026 Ella Networks

package db

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	DBQueriesTotal  *prometheus.CounterVec
	DBQueryDuration *prometheus.HistogramVec
)

// ipPoolCollector implements prometheus.Collector to provide per-pool utilization
// gauges that are computed on each scrape.
type ipPoolCollector struct {
	db           *Database
	usedDesc     *prometheus.Desc
	totalDesc    *prometheus.Desc
	poolSizeFunc func(ipPool string, id int) int
}

func newIPPoolCollector(db *Database, poolSizeFunc func(string, int) int) *ipPoolCollector {
	return &ipPoolCollector{
		db:           db,
		poolSizeFunc: poolSizeFunc,
		usedDesc: prometheus.NewDesc(
			"app_ip_pool_addresses_used",
			"Current number of allocated IP addresses in the pool",
			[]string{"pool", "address_family"}, nil,
		),
		totalDesc: prometheus.NewDesc(
			"app_ip_pool_addresses_total",
			"Total capacity of the IP address pool",
			[]string{"pool", "address_family"}, nil,
		),
	}
}

func (c *ipPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.usedDesc

	ch <- c.totalDesc
}

func (c *ipPoolCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	dataNetworks, err := c.db.ListAllDataNetworks(ctx)
	if err != nil {
		logger.MetricsLog.Warn("Failed to list data networks for pool metrics", zap.Error(err))
		return
	}

	for _, dn := range dataNetworks {
		if dn.IPPool == "" {
			continue
		}

		total := c.poolSizeFunc(dn.IPPool, dn.ID)

		used, err := c.db.CountLeasesByPool(ctx, dn.ID)
		if err != nil {
			logger.MetricsLog.Warn("Failed to count leases for pool", zap.String("pool", dn.Name), zap.Error(err))
			continue
		}

		ch <- prometheus.MustNewConstMetric(c.usedDesc, prometheus.GaugeValue, float64(used), dn.Name, "ipv4")

		ch <- prometheus.MustNewConstMetric(c.totalDesc, prometheus.GaugeValue, float64(total), dn.Name, "ipv4")
	}
}

func RegisterMetrics(db *Database) {
	if DBQueryDuration != nil {
		// Already registered, skip
		return
	}

	databaseStorageUsed := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
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

	ipAddressesTotal := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
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

	ipAddressesAllocated := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
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

	DBQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_database_queries_total",
			Help: "Total number of database queries by table and operation",
		},
		[]string{"table", "operation"},
	)

	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "app_database_query_duration_seconds",
			Help:    "Duration of database queries",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0}, // 1ms to 1s
		},
		[]string{"table", "operation"},
	)

	prometheus.MustRegister(databaseStorageUsed)
	prometheus.MustRegister(ipAddressesTotal)
	prometheus.MustRegister(ipAddressesAllocated)
	prometheus.MustRegister(DBQueryDuration)
	prometheus.MustRegister(DBQueriesTotal)
}

// RegisterPoolMetrics registers per-pool Prometheus gauges. poolSizeFunc
// computes the total usable addresses for a given CIDR and data-network ID.
// Must be called after RegisterMetrics.
func RegisterPoolMetrics(db *Database, poolSizeFunc func(string, int) int) {
	prometheus.MustRegister(newIPPoolCollector(db, poolSizeFunc))
}

func (db *Database) GetSize() (int64, error) {
	fileInfo, err := os.Stat(db.filepath)
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

func (db *Database) GetIPAddressesTotal() (int, error) {
	dataNetworks, _, err := db.ListDataNetworksPage(context.Background(), 1, 1000)
	if err != nil {
		return 0, err
	}

	var total int

	for _, dn := range dataNetworks {
		ipPool := dn.IPPool

		_, ipNet, err := net.ParseCIDR(ipPool)
		if err != nil {
			return 0, fmt.Errorf("invalid IP pool format '%s': %v", ipPool, err)
		}

		total += countIPsInCIDR(ipNet)
	}

	return total, nil
}

func countIPsInCIDR(ipNet *net.IPNet) int {
	ones, bits := ipNet.Mask.Size()
	if bits-ones > 30 {
		return int(^uint32(0))
	}

	return 1 << (bits - ones)
}

func (db *Database) GetIPAddressesAllocated(ctx context.Context) (int, error) {
	count, err := db.CountActiveLeases(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count active leases: %v", err)
	}

	return count, nil
}
