// Copyright 2023 Edgecom LLC
// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Modified by Ella Networks.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ebpf

import (
	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// Index of the per-action forwarding counter (enum ctx_action in
// bpf/ctx/ctx.h). The values coincide with the XDP verdict encoding, which is
// why an XDP program's return value can be compared against them directly.
const (
	ActionAborted  = 0
	ActionDrop     = 1
	ActionPass     = 2
	ActionTx       = 3
	ActionRedirect = 4

	// UPFMaxAction is the width of the forwarding counter array.
	UPFMaxAction = 8
)

// Direction selects the statistics map: the pipelines keep one each.
type Direction string

const (
	Uplink   Direction = "uplink"
	Downlink Direction = "downlink"
)

func statsMap(bpfObjects *BpfObjects, dir Direction) *ebpf.Map {
	if dir == Uplink {
		return bpfObjects.UplinkStatistics
	}

	return bpfObjects.DownlinkStatistics
}

// readStats sums one direction's statistics over CPUs, returning false when
// the map could not be read.
func readStats(bpfObjects *BpfObjects, dir Direction) (N3N6EntrypointUpfStatistic, bool) {
	var (
		perCPU []N3N6EntrypointUpfStatistic
		total  N3N6EntrypointUpfStatistic
	)

	if err := statsMap(bpfObjects, dir).Lookup(uint32(0), &perCPU); err != nil {
		logger.UpfLog.Warn("failed to fetch UPF datapath stats",
			zap.String("direction", string(dir)), zap.Error(err))

		return total, false
	}

	for _, s := range perCPU {
		total.ByteCounter.Bytes += s.ByteCounter.Bytes
		total.PacketCounters.Rx += s.PacketCounters.Rx
		total.PacketCounters.Tx += s.PacketCounters.Tx

		for i := range s.ForwardedActions {
			total.ForwardedActions[i] += s.ForwardedActions[i]
		}

		for i := range s.DropReasons {
			total.DropReasons[i] += s.DropReasons[i]
		}
	}

	return total, true
}

// DatapathCounters is one direction's packet accounting.
type DatapathCounters struct {
	Forwarded [UPFMaxAction]uint64
	Dropped   [UPFDropReasonMax]uint64
}

// A direction whose map could not be read is absent rather than zero.
func GetDatapathCounters(bpfObjects *BpfObjects) map[Direction]DatapathCounters {
	out := make(map[Direction]DatapathCounters, 2)

	for _, dir := range []Direction{Uplink, Downlink} {
		s, ok := readStats(bpfObjects, dir)
		if !ok {
			continue
		}

		out[dir] = DatapathCounters{Forwarded: s.ForwardedActions, Dropped: s.DropReasons}
	}

	return out
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
	FibError4       uint64
	FibError6       uint64
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
		rs.FibError4 += s.FibLookupIp4Error
		rs.FibError6 += s.FibLookupIp6Error
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

// UPFDropReasonMax is the width of the drop counter array.
const UPFDropReasonMax = 64

// Label value per drop reason, in the order of enum upf_drop_reason in
// bpf/utils/drop_reason.h. TestDropReasonNamesMatchDatapath fails if the two
// drift apart.
var dropReasonNames = [...]string{
	"unspecified",
	"no_uplink_session",
	"no_downlink_session",
	"far_no_forward",
	"far_no_encap",
	"far_unsupported",
	"qer_gate_closed",
	"qer_rate_limit",
	"sdf_filter",
	"nocp_buffer",
	"source_spoof_ipv4",
	"source_spoof_ipv6",
	"decap_family_mismatch",
	"encap_gso",
	"df_not_set",
	"rs_intercepted",
	"nat_unsolicited",
	"nat_fragment",
	"nat_port_exhausted",
	"nat_unsupported_proto",
	"nat_malformed",
	"nat_translate_failed",
	"fib_no_neigh",
	"fib_blackhole",
	"fib_unreachable",
	"fib_prohibit",
	"fib_no_src_addr",
	"fib_frag_needed",
	"fib_error",
	"ifindex_mismatch",
	"malformed_gtp",
	"malformed_header",
	"internal_pull_failed",
	"internal_mtu_check_failed",
	"internal_encap_failed",
	"internal_decap_failed",
	"internal_csum_failed",
	"internal_resize_failed",
	"internal_write_failed",
	"internal_map_lookup_failed",
	"internal_tx_failed",
}

// DropReasonNames returns every reason's label value, indexed by reason.
func DropReasonNames() []string { return dropReasonNames[:] }

// DropReasonByName resolves a label value to its index.
func DropReasonByName(name string) (int, bool) {
	for i, n := range dropReasonNames {
		if n == name {
			return i, true
		}
	}

	return 0, false
}

// DropCount returns how many frames one direction did not forward for one
// reason; an unknown name returns 0.
func DropCount(bpfObjects *BpfObjects, dir Direction, reason string) uint64 {
	i, ok := DropReasonByName(reason)
	if !ok {
		return 0
	}

	s, ok := readStats(bpfObjects, dir)
	if !ok {
		return 0
	}

	return s.DropReasons[i]
}

// ForwardCount counts frames forwarded with one action.
func ForwardCount(bpfObjects *BpfObjects, dir Direction, action int) uint64 {
	s, ok := readStats(bpfObjects, dir)
	if !ok {
		return 0
	}

	return s.ForwardedActions[action]
}

// TotalDrops counts every frame one direction did not forward.
func TotalDrops(bpfObjects *BpfObjects, dir Direction) uint64 {
	s, ok := readStats(bpfObjects, dir)
	if !ok {
		return 0
	}

	var total uint64
	for _, v := range s.DropReasons {
		total += v
	}

	return total
}
