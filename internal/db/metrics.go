// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	DBQueriesTotal  *prometheus.CounterVec
	DBQueryDuration *prometheus.HistogramVec
)

const (
	metricsCollectTimeout = 2 * time.Second
	dataNetworksPageSize  = 1000
)

type metricsCollector struct {
	db *Database

	storageDesc     *prometheus.Desc
	ipTotalDesc     *prometheus.Desc
	ipAllocatedDesc *prometheus.Desc
}

func newMetricsCollector(db *Database) *metricsCollector {
	return &metricsCollector{
		db: db,
		storageDesc: prometheus.NewDesc(
			"app_database_storage_bytes",
			"Storage used by the Ella Core SQLite database file on disk, in bytes.",
			nil,
			nil,
		),
		ipTotalDesc: prometheus.NewDesc(
			"app_ip_addresses_total",
			"The total number of IP addresses available for subscribers",
			nil,
			nil,
		),
		ipAllocatedDesc: prometheus.NewDesc(
			"app_ip_addresses_allocated_total",
			"The total number of IP addresses currently allocated to subscribers",
			nil,
			nil,
		),
	}
}

func (c *metricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.storageDesc

	ch <- c.ipTotalDesc

	ch <- c.ipAllocatedDesc
}

func (c *metricsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), metricsCollectTimeout)
	defer cancel()

	size, err := c.db.GetSize()
	if err != nil {
		logger.MetricsLog.Warn("Failed to get database storage used", zap.Error(err))
		metrics.CollectionError(metrics.CollectorDatabaseStorage)
	} else {
		ch <- prometheus.MustNewConstMetric(c.storageDesc, prometheus.GaugeValue, float64(size))
	}

	total, err := c.db.GetIPAddressesTotal(ctx)
	if err != nil {
		logger.MetricsLog.Warn("Failed to get total IP addresses", zap.Error(err))
		metrics.CollectionError(metrics.CollectorDatabaseIPTotal)
	} else {
		ch <- prometheus.MustNewConstMetric(c.ipTotalDesc, prometheus.GaugeValue, float64(total))
	}

	allocated, err := c.db.GetIPAddressesAllocated(ctx)
	if err != nil {
		logger.MetricsLog.Warn("Failed to get allocated IP addresses", zap.Error(err))
		metrics.CollectionError(metrics.CollectorDatabaseIPAllocated)
	} else {
		ch <- prometheus.MustNewConstMetric(c.ipAllocatedDesc, prometheus.GaugeValue, float64(allocated))
	}
}

func RegisterMetrics(db *Database) {
	if DBQueryDuration != nil {
		// Already registered, skip
		return
	}

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

	prometheus.MustRegister(newMetricsCollector(db))
	prometheus.MustRegister(DBQueryDuration)
	prometheus.MustRegister(DBQueriesTotal)
}

// GetSize returns the on-disk size of the database file in bytes.
func (db *Database) GetSize() (int64, error) {
	fileInfo, err := os.Stat(db.Path())
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

func (db *Database) GetIPAddressesTotal(ctx context.Context) (int, error) {
	var total int

	for page := 1; ; page++ {
		dataNetworks, count, err := db.ListDataNetworksPage(ctx, page, dataNetworksPageSize)
		if err != nil {
			return 0, err
		}

		for _, dn := range dataNetworks {
			ipv4Pool := dn.IPv4Pool

			prefix, err := netip.ParsePrefix(ipv4Pool)
			if err != nil {
				return 0, fmt.Errorf("invalid IP pool format '%s': %v", ipv4Pool, err)
			}

			total += countIPsInPrefix(prefix)
		}

		if len(dataNetworks) == 0 || page*dataNetworksPageSize >= count {
			return total, nil
		}
	}
}

func countIPsInPrefix(prefix netip.Prefix) int {
	bits := prefix.Bits()
	if 32-bits > 30 {
		return int(^uint32(0))
	}

	return 1 << (32 - bits)
}

func (db *Database) GetIPAddressesAllocated(ctx context.Context) (int, error) {
	count, err := db.CountActiveLeases(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count active leases: %v", err)
	}

	return count, nil
}
