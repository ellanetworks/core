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

type RouteStats struct {
	FibSuccess      uint64
	FibNoNeigh      uint64
	FibBlackhole    uint64
	FibUnreachable  uint64
	FibProhibit     uint64
	FibNoSrcAddr    uint64
	FibFragNeeded   uint64
	FibNotFwded     uint64
	FibFwdDisabled  uint64
	FibUnsuppLwt    uint64
	IfindexMismatch uint64
}

func aggregateRouteStats(perCPUStats []N3N6EntrypointRouteStat) RouteStats {
	var rs RouteStats
	for _, s := range perCPUStats {
		rs.FibSuccess += s.FibLookupIp4Success + s.FibLookupIp6Success
		rs.FibNoNeigh += s.FibLookupIp4NoNeigh + s.FibLookupIp6NoNeigh
		rs.FibBlackhole += s.FibLookupIp4Blackhole + s.FibLookupIp6Blackhole
		rs.FibUnreachable += s.FibLookupIp4Unreachable + s.FibLookupIp6Unreachable
		rs.FibProhibit += s.FibLookupIp4Prohibit + s.FibLookupIp6Prohibit
		rs.FibNoSrcAddr += s.FibLookupIp4NoSrcAddr + s.FibLookupIp6NoSrcAddr
		rs.FibFragNeeded += s.FibLookupIp4FragNeeded + s.FibLookupIp6FragNeeded
		rs.FibNotFwded += s.FibLookupIp4NotFwded + s.FibLookupIp6NotFwded
		rs.FibFwdDisabled += s.FibLookupIp4FwdDisabled + s.FibLookupIp6FwdDisabled
		rs.FibUnsuppLwt += s.FibLookupIp4UnsuppLwt + s.FibLookupIp6UnsuppLwt
		rs.IfindexMismatch += s.Ip4IfindexMismatch + s.Ip6IfindexMismatch
	}

	return rs
}

func GetN3RouteStats(bpfObjects *BpfObjects) RouteStats {
	var stats []N3N6EntrypointRouteStat

	err := bpfObjects.UplinkRouteStats.Lookup(uint32(0), &stats)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N3 route stats", zap.Error(err))
		return RouteStats{}
	}

	return aggregateRouteStats(stats)
}

func GetN6RouteStats(bpfObjects *BpfObjects) RouteStats {
	var stats []N3N6EntrypointRouteStat

	err := bpfObjects.DownlinkRouteStats.Lookup(uint32(0), &stats)
	if err != nil {
		logger.UpfLog.Warn("failed to fetch UPF N6 route stats", zap.Error(err))
		return RouteStats{}
	}

	return aggregateRouteStats(stats)
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
