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

// ProfileIndex mirrors the profile_index enum in profiling.h.
// The values must stay in sync with the C enum.
const (
	ProfN3Total        = 0
	ProfN6Total        = 1
	ProfN3PdrLookup    = 2
	ProfN6PdrLookup    = 3
	ProfN3MtuCheck     = 4
	ProfN6MtuCheck     = 5
	ProfN3QerRatelimit = 6
	ProfN6QerRatelimit = 7
	ProfN3GtpManip     = 8
	ProfN6GtpManip     = 9
	ProfN3SdfFilter    = 10
	ProfN6SdfFilter    = 11
	ProfN3Nat          = 12
	ProfN6Nat          = 13
	ProfN3FibRouting   = 14
	ProfN6FibRouting   = 15
	ProfNumEntries     = 16
)

// ProfileEntry holds the aggregated (across all CPUs) profiling data for one
// pipeline sub-stage.
type ProfileEntry struct {
	TotalNs uint64
	Count   uint64
}

// bpfProfileEntry mirrors the C struct profile_entry { __u64 total_ns; __u64 count; }
// from profiling.h. Using a local type avoids a direct reference to the
// bpf2go-generated N3N6EntrypointProfileEntry, which is only emitted when the
// BPF code is compiled with -DENABLE_PROFILING.
type bpfProfileEntry struct {
	TotalNs uint64
	Count   uint64
}

// ReadProfilingStats reads the per-CPU profiling_map and aggregates all
// entries across CPUs. It returns a slice of ProfNumEntries elements indexed
// by the ProfileIndex constants above.
//
// If the map is not present (i.e. the BPF program was compiled without
// -DENABLE_PROFILING) the function returns nil, nil.
func ReadProfilingStats(bpfObjects *BpfObjects) ([]ProfileEntry, error) {
	if bpfObjects == nil || bpfObjects.ProfilingMap == nil {
		return nil, nil
	}

	results := make([]ProfileEntry, ProfNumEntries)

	for i := uint32(0); i < ProfNumEntries; i++ {
		var perCPU []bpfProfileEntry
		if err := bpfObjects.ProfilingMap.Lookup(i, &perCPU); err != nil {
			logger.UpfLog.Warn("failed to read profiling map", zap.Uint32("index", i), zap.Error(err))
			continue
		}

		var totalNs, count uint64
		for _, e := range perCPU {
			totalNs += e.TotalNs
			count += e.Count
		}

		results[i] = ProfileEntry{
			TotalNs: totalNs,
			Count:   count,
		}
	}

	return results, nil
}
