// Copyright 2026 Ella Networks

package db

import (
	"context"
	"fmt"
	"net/netip"
	"os"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	DBQueriesTotal  *prometheus.CounterVec
	DBQueryDuration *prometheus.HistogramVec
)

func RegisterMetrics(db *Database) {
	if DBQueryDuration != nil {
		// Already registered, skip
		return
	}

	sharedDBStorageUsed := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "app_database_storage_bytes",
		Help:        "Storage used by an Ella Core SQLite database file on disk, in bytes.",
		ConstLabels: prometheus.Labels{"database": "shared"},
	}, func() float64 {
		size, err := db.GetSharedSize()
		if err != nil {
			logger.MetricsLog.Warn("Failed to get shared database storage used", zap.Error(err))
			return 0
		}

		return float64(size)
	})

	localDBStorageUsed := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "app_database_storage_bytes",
		Help:        "Storage used by an Ella Core SQLite database file on disk, in bytes.",
		ConstLabels: prometheus.Labels{"database": "local"},
	}, func() float64 {
		size, err := db.GetLocalSize()
		if err != nil {
			logger.MetricsLog.Warn("Failed to get local database storage used", zap.Error(err))
			return 0
		}

		return float64(size)
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

	prometheus.MustRegister(sharedDBStorageUsed)
	prometheus.MustRegister(localDBStorageUsed)
	prometheus.MustRegister(ipAddressesTotal)
	prometheus.MustRegister(ipAddressesAllocated)
	prometheus.MustRegister(DBQueryDuration)
	prometheus.MustRegister(DBQueriesTotal)
}

// GetSharedSize returns the on-disk size of shared.db in bytes.
func (db *Database) GetSharedSize() (int64, error) {
	fileInfo, err := os.Stat(db.SharedPath())
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

// GetLocalSize returns the on-disk size of local.db in bytes.
func (db *Database) GetLocalSize() (int64, error) {
	fileInfo, err := os.Stat(db.LocalPath())
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

		prefix, err := netip.ParsePrefix(ipPool)
		if err != nil {
			return 0, fmt.Errorf("invalid IP pool format '%s': %v", ipPool, err)
		}

		total += countIPsInPrefix(prefix)
	}

	return total, nil
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
