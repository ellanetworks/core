package ebpf

import (
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	XDP_ABORTED  = 0
	XDP_DROP     = 1
	XDP_PASS     = 2
	XDP_TX       = 3
	XDP_REDIRECT = 4
)

func getUpfN3XdpStatisticField(bpfObjects *BpfObjects, field uint32) uint64 {
	var statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.UplinkStatistics.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N3 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range statistics {
		totalValue += statistic.XdpActions[field]
	}

	return totalValue
}

func getUpfN6XdpStatisticField(bpfObjects *BpfObjects, field uint32) uint64 {
	var statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.DownlinkStatistics.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N6 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range statistics {
		totalValue += statistic.XdpActions[field]
	}

	return totalValue
}

func GetN3Aborted(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_ABORTED)
}

func GetN3Drop(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_DROP)
}

func GetN3Pass(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_PASS)
}

func GetN3Tx(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_TX)
}

func GetN3Redirect(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_REDIRECT)
}

func GetN6Aborted(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_ABORTED)
}

func GetN6Drop(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_DROP)
}

func GetN6Pass(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_PASS)
}

func GetN6Tx(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_TX)
}

func GetN6Redirect(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_REDIRECT)
}

// FIB lookup result getters — N3 (uplink)

func getN3RouteStats(bpfObjects *BpfObjects) []N3N6EntrypointRouteStat {
	var stats []N3N6EntrypointRouteStat

	err := bpfObjects.UplinkRouteStats.Lookup(uint32(0), &stats)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N3 route stats", zap.Error(err))
		return nil
	}

	return stats
}

func GetN3FibOk(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4Ok + s.FibLookupIp6Ok
	}

	return total
}

func GetN3FibDrop(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4ErrorDrop + s.FibLookupIp6ErrorDrop
	}

	return total
}

func GetN3FibPass(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4ErrorPass + s.FibLookupIp6ErrorPass
	}

	return total
}

func GetN3NoNeigh(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4NoNeigh + s.FibLookupIp6NoNeigh
	}

	return total
}

// FIB lookup result getters — N6 (downlink)

func getN6RouteStats(bpfObjects *BpfObjects) []N3N6EntrypointRouteStat {
	var stats []N3N6EntrypointRouteStat

	err := bpfObjects.DownlinkRouteStats.Lookup(uint32(0), &stats)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N6 route stats", zap.Error(err))
		return nil
	}

	return stats
}

func GetN6FibOk(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4Ok + s.FibLookupIp6Ok
	}

	return total
}

func GetN6FibDrop(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4ErrorDrop + s.FibLookupIp6ErrorDrop
	}

	return total
}

func GetN6FibPass(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4ErrorPass + s.FibLookupIp6ErrorPass
	}

	return total
}

func GetN6NoNeigh(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4NoNeigh + s.FibLookupIp6NoNeigh
	}

	return total
}

func GetN3UplinkThroughputStats(bpfObjects *BpfObjects) uint64 {
	var n3Statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.UplinkStatistics.Lookup(uint32(0), &n3Statistics)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N3 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range n3Statistics {
		totalValue += statistic.ByteCounter.Bytes
	}

	return totalValue
}

func GetN6DownlinkThroughputStats(bpfObjects *BpfObjects) uint64 {
	var n6Statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.DownlinkStatistics.Lookup(uint32(0), &n6Statistics)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N6 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range n6Statistics {
		totalValue += statistic.ByteCounter.Bytes
	}

	return totalValue
}
