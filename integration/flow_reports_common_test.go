// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"strconv"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	timestampUpperBuffer = 90 * time.Second

	probeRoundtrips = 3

	ipProtoICMP   uint8 = 1
	ipProtoTCP    uint8 = 6
	ipProtoUDP    uint8 = 17
	ipProtoICMPv6 uint8 = 58

	responderPort = scenarios.DefaultProbePort

	bytesPerICMPPacketIPv4 = 98  // 14 (Eth) + 20 (IP) + 8 (ICMP) + 56 payload
	bytesPerICMPPacketIPv6 = 118 // 14 + 40 + 8 + 56
	bytesPerUDPPacketIPv4  = 59  // 14 + 20 + 8 + 17
	bytesPerUDPPacketIPv6  = 79  // 14 + 40 + 8 + 17

	bytesPerHandshakePacketIPv4 = 74 // 14 + 20 + 40
	bytesPerHandshakePacketIPv6 = 94 // 14 + 40 + 40

	probeSourcePortBase      = 20000
	probeSourcePortStride    = 100
	smokeProbeSourcePortBase = 21000

	tcpDropPacketsPerTupleDownlink = 7
	tcpDropPacketsPerTupleUplink   = 2

	tcpAllowPacketsPerIMSILo = 12
	tcpAllowPacketsPerIMSIHi = 40
	tcpAllowBytesPerIMSILoV4 = 600
	tcpAllowBytesPerIMSIHiV4 = 3500
	tcpAllowBytesPerIMSILoV6 = 900
	tcpAllowBytesPerIMSIHiV6 = 4500
)

// ipFamilyParams holds the values needed to drive a flow-report test
// against either an IPv4 or IPv6 PDU session.
type ipFamilyParams struct {
	family            IPFamily
	scenarioAllowed   string
	scenarioBlocked   string
	pingDestination   string
	uePool            string
	nonMatchingPrefix string
	hostPrefix        string // "/32" for IPv4, "/128" for IPv6
}

// probeProtocolParams holds the per-protocol numbers needed to compose
// flow-report predicates for a given (family, protocol) combination.
//
// ICMP and UDP exchange fixed-length payloads, so per-flow packet and
// byte counts are exact. TCP per-flow shape varies with kernel
// delayed-ACK timing — see the tcp* constants and the tcp branch in
// expectedFlowsContentPredicate.
type probeProtocolParams struct {
	name                 string
	ipProto              uint8
	flowsPerUE           int
	packetsPerFlow       *uint64
	bytesPerFlowUplink   *uint64
	bytesPerFlowDownlink *uint64
	supportsPortRules    bool
}

// familyParams picks the parameter set matching the active IP family for the
// given RAN (ranPrefix is the scenario namespace: "gnb" for 5G, "s1enb" for 4G).
// The probe machinery, byte/packet constants, and rule shapes are RAT-agnostic;
// only the connectivity_expect scenario names differ. DualStack reuses the IPv4
// leg, matching the convention used elsewhere in the integration suite.
func familyParams(family IPFamily, ranPrefix string) ipFamilyParams {
	if family == IPv6Only {
		return ipFamilyParams{
			family:            IPv6Only,
			scenarioAllowed:   ranPrefix + "/connectivity_expect_allowed_ipv6",
			scenarioBlocked:   ranPrefix + "/connectivity_expect_blocked_ipv6",
			pingDestination:   scenarios.DefaultPingDestinationV6,
			uePool:            scenarios.DefaultUEIPv6Pool,
			nonMatchingPrefix: "2001:db8:dead::/48",
			hostPrefix:        "/128",
		}
	}

	return ipFamilyParams{
		family:            family,
		scenarioAllowed:   ranPrefix + "/connectivity_expect_allowed",
		scenarioBlocked:   ranPrefix + "/connectivity_expect_blocked",
		pingDestination:   scenarios.DefaultPingDestination,
		uePool:            scenarios.DefaultUEIPv4Pool,
		nonMatchingPrefix: "203.0.113.0/24",
		hostPrefix:        "/32",
	}
}

// protocolParams returns the per-protocol numbers for the given
// family + protocol pair. Protocol must be one of "icmp", "tcp",
// "udp".
func protocolParams(family IPFamily, protocol string) probeProtocolParams {
	switch protocol {
	case "tcp":
		return probeProtocolParams{
			name:              "tcp",
			ipProto:           ipProtoTCP,
			flowsPerUE:        probeRoundtrips,
			supportsPortRules: true,
		}
	case "udp":
		packets := uint64(probeRoundtrips)

		perPacket := uint64(bytesPerUDPPacketIPv4)
		if family == IPv6Only {
			perPacket = uint64(bytesPerUDPPacketIPv6)
		}

		perFlow := packets * perPacket

		return probeProtocolParams{
			name:                 "udp",
			ipProto:              ipProtoUDP,
			flowsPerUE:           1,
			packetsPerFlow:       &packets,
			bytesPerFlowUplink:   &perFlow,
			bytesPerFlowDownlink: &perFlow,
			supportsPortRules:    true,
		}
	default: // icmp
		ipProto := ipProtoICMP
		bytes := uint64(bytesPerICMPPacketIPv4)

		if family == IPv6Only {
			ipProto = ipProtoICMPv6
			bytes = uint64(bytesPerICMPPacketIPv6)
		}

		packets := uint64(probeRoundtrips)
		perFlow := packets * bytes

		return probeProtocolParams{
			name:                 "icmp",
			ipProto:              ipProto,
			flowsPerUE:           1,
			packetsPerFlow:       &packets,
			bytesPerFlowUplink:   &perFlow,
			bytesPerFlowDownlink: &perFlow,
			supportsPortRules:    false,
		}
	}
}

// expectedBytesPerFlow returns the exact per-flow byte count to assert
// for the given direction, or nil when the protocol's per-flow bytes
// aren't a fixed scalar (variable-length payloads, TCP).
func expectedBytesPerFlow(pp probeProtocolParams, direction string) *uint64 {
	if direction == "uplink" {
		return pp.bytesPerFlowUplink
	}

	return pp.bytesPerFlowDownlink
}

// apiSourceIPFilter returns the value to pass as the flow-report
// Source query parameter for the given direction. On IPv6 this
// excludes RA/RS background traffic.
func apiSourceIPFilter(direction string, fp ipFamilyParams) string {
	if direction == "downlink" {
		return fp.pingDestination
	}

	return ""
}

// apiDestinationIPFilter is the Destination-side counterpart to
// apiSourceIPFilter.
func apiDestinationIPFilter(direction string, fp ipFamilyParams) string {
	if direction == "uplink" {
		return fp.pingDestination
	}

	return ""
}

// expectedFlowsContentPredicate composes the per-direction predicate
// for the polling loop.
//
// TCP uses per-IMSI aggregate bounds because the kernel can split one
// connection's records across observation windows and packet counts
// are kernel-dependent. The strict shape is still asserted: every
// IMSI must appear with exactly probeRoundtrips distinct ephemeral
// (sport, dport) tuples and no single tuple can be backed by more
// than two records (one normal record plus at most one split).
//
// ICMP and UDP keep exact per-flow assertions: their per-flow shape
// is deterministic (or bounded with a tight range for IPv6 UDP
// downlink, see bytesPerFlowDownlinkLo/Hi).
func expectedFlowsContentPredicate(direction, action string, expectedIMSIs []string, fp ipFamilyParams, pp probeProtocolParams) fixture.FlowReportPredicate {
	preds := []fixture.FlowReportPredicate{
		fixture.EachAction(action),
		fixture.EachDirection(direction),
		fixture.EachProtocol(pp.ipProto),
	}

	switch direction {
	case "uplink":
		preds = append(preds,
			fixture.EachSourceIPInCIDR(fp.uePool),
			fixture.EachDestinationIPIs(fp.pingDestination),
		)
	case "downlink":
		preds = append(preds,
			fixture.EachSourceIPIs(fp.pingDestination),
			fixture.EachDestinationIPInCIDR(fp.uePool),
		)
	}

	if pp.name == "tcp" {
		preds = append(preds,
			fixture.DistinctImsis(len(expectedIMSIs)),
			fixture.EachIMSIDistinctTuplesIs(probeRoundtrips),
			fixture.EachTupleHasAtMost(2),
		)

		if action == "drop" {
			maxPackets := uint64(tcpDropPacketsPerTupleDownlink)
			if direction == "uplink" {
				maxPackets = tcpDropPacketsPerTupleUplink
			}

			preds = append(preds,
				fixture.EachBytesPerPacketIs(handshakeBytes(fp)),
				fixture.EachTuplePacketsAtMost(maxPackets),
			)
		} else {
			pktLo, pktHi, bytesLo, bytesHi := tcpAllowPerIMSIBounds(fp)

			preds = append(preds,
				fixture.EachIMSITotalPacketsInRange(pktLo, pktHi),
				fixture.EachIMSITotalBytesInRange(bytesLo, bytesHi),
			)
		}
	} else {
		expandedIMSIs := repeatIMSIs(expectedIMSIs, pp.flowsPerUE)

		preds = append(preds,
			fixture.Count(len(expandedIMSIs)),
			fixture.ImsisAre(expandedIMSIs),
		)
		if pp.packetsPerFlow != nil {
			preds = append(preds, fixture.EachPackets(*pp.packetsPerFlow))
		}
	}

	return fixture.And(preds...)
}

func handshakeBytes(fp ipFamilyParams) uint64 {
	if fp.family == IPv6Only {
		return bytesPerHandshakePacketIPv6
	}

	return bytesPerHandshakePacketIPv4
}

func probeTupleScope(direction string, pp probeProtocolParams, srcPortBase, ueCount int) fixture.FlowReportScope {
	if pp.name != "tcp" || srcPortBase == 0 {
		return nil
	}

	lo := uint16(srcPortBase)
	hi := uint16(srcPortBase + ueCount*probeRoundtrips - 1)

	return func(f client.FlowReport) bool {
		uePort := f.DestinationPort
		if direction == "uplink" {
			uePort = f.SourcePort
		}

		return uePort >= lo && uePort <= hi
	}
}

func tcpAllowPerIMSIBounds(fp ipFamilyParams) (pktLo, pktHi, byteLo, byteHi uint64) {
	pktLo, pktHi = tcpAllowPacketsPerIMSILo, tcpAllowPacketsPerIMSIHi

	if fp.family == IPv6Only {
		byteLo, byteHi = tcpAllowBytesPerIMSILoV6, tcpAllowBytesPerIMSIHiV6
	} else {
		byteLo, byteHi = tcpAllowBytesPerIMSILoV4, tcpAllowBytesPerIMSIHiV4
	}

	return
}

// apiProtocolFilter returns the string protocol value to pass on the
// flow-report list query.
func apiProtocolFilter(pp probeProtocolParams) string {
	return strconv.Itoa(int(pp.ipProto))
}

// scenarioRunArgs mirrors TestIntegrationTester's pattern for building
// core-tester CLI args: --ip-version is injected only for scenarios
// that are explicitly family-restricted, then --protocol, then any
// fixture-supplied extras.
func scenarioRunArgs(name string, spec scenarios.FixtureSpec, pp probeProtocolParams, srcPortBase int) []string {
	var args []string

	if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok {
		args = append(args, "--ip-version", string(requiredFamily))
	}

	args = append(args, "--protocol", pp.name)

	if srcPortBase != 0 {
		args = append(args, "--probe-source-port-base", strconv.Itoa(srcPortBase))
	}

	args = append(args, spec.ExtraArgs...)

	return args
}

// repeatIMSIs returns a slice where each input IMSI appears n times,
// preserving the input order.
func repeatIMSIs(imsis []string, n int) []string {
	if n <= 1 {
		return imsis
	}

	out := make([]string, 0, len(imsis)*n)
	for _, imsi := range imsis {
		for i := 0; i < n; i++ {
			out = append(out, imsi)
		}
	}

	return out
}
