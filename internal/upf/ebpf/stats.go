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

func GetN3FibSuccess(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4Success + s.FibLookupIp6Success
	}

	return total
}

func GetN3FibNoNeigh(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4NoNeigh + s.FibLookupIp6NoNeigh
	}

	return total
}

func GetN3FibBlackhole(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4Blackhole + s.FibLookupIp6Blackhole
	}

	return total
}

func GetN3FibUnreachable(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4Unreachable + s.FibLookupIp6Unreachable
	}

	return total
}

func GetN3FibProhibit(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4Prohibit + s.FibLookupIp6Prohibit
	}

	return total
}

func GetN3FibNoSrcAddr(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4NoSrcAddr + s.FibLookupIp6NoSrcAddr
	}

	return total
}

func GetN3FibFragNeeded(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4FragNeeded + s.FibLookupIp6FragNeeded
	}

	return total
}

func GetN3FibNotFwded(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4NotFwded + s.FibLookupIp6NotFwded
	}

	return total
}

func GetN3FibFwdDisabled(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4FwdDisabled + s.FibLookupIp6FwdDisabled
	}

	return total
}

func GetN3FibUnsuppLwt(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.FibLookupIp4UnsuppLwt + s.FibLookupIp6UnsuppLwt
	}

	return total
}

func GetN3IfindexMismatch(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN3RouteStats(bpfObjects) {
		total += s.Ip4IfindexMismatch + s.Ip6IfindexMismatch
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

func GetN6FibSuccess(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4Success + s.FibLookupIp6Success
	}

	return total
}

func GetN6FibNoNeigh(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4NoNeigh + s.FibLookupIp6NoNeigh
	}

	return total
}

func GetN6FibBlackhole(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4Blackhole + s.FibLookupIp6Blackhole
	}

	return total
}

func GetN6FibUnreachable(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4Unreachable + s.FibLookupIp6Unreachable
	}

	return total
}

func GetN6FibProhibit(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4Prohibit + s.FibLookupIp6Prohibit
	}

	return total
}

func GetN6FibNoSrcAddr(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4NoSrcAddr + s.FibLookupIp6NoSrcAddr
	}

	return total
}

func GetN6FibFragNeeded(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4FragNeeded + s.FibLookupIp6FragNeeded
	}

	return total
}

func GetN6FibNotFwded(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4NotFwded + s.FibLookupIp6NotFwded
	}

	return total
}

func GetN6FibFwdDisabled(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4FwdDisabled + s.FibLookupIp6FwdDisabled
	}

	return total
}

func GetN6FibUnsuppLwt(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.FibLookupIp4UnsuppLwt + s.FibLookupIp6UnsuppLwt
	}

	return total
}

func GetN6IfindexMismatch(bpfObjects *BpfObjects) uint64 {
	var total uint64

	for _, s := range getN6RouteStats(bpfObjects) {
		total += s.Ip4IfindexMismatch + s.Ip6IfindexMismatch
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
