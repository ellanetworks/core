package integration_test

import (
	"strconv"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	// Tolerance covering clock skew between the kernel-derived flow
	// timestamps and the test's wall clock.
	timestampUpperBuffer = 5 * time.Second

	// Probe round-trip count, matching probeAttemptCount in the
	// scenarios package. ICMP echoes and UDP datagrams stay in a
	// single 5-tuple; TCP opens this many distinct connections.
	probeRoundtrips = 3

	// IP-proto numbers used by the SDF filter and reported in flow
	// records.
	ipProtoICMP   uint8 = 1
	ipProtoTCP    uint8 = 6
	ipProtoUDP    uint8 = 17
	ipProtoICMPv6 uint8 = 58

	// Listening port of the responder on N6.
	responderPort = scenarios.DefaultProbePort

	// Per-packet byte counts after the XDP path strips GTP/UDP/IP
	// outer headers and rewrites L2. Symmetric uplink/downlink.
	bytesPerICMPPacketIPv4 = 98  // 14 (Eth) + 20 (IP) + 8 (ICMP) + 56 payload
	bytesPerICMPPacketIPv6 = 118 // 14 + 40 + 8 + 56
	bytesPerUDPPacketIPv4  = 59  // 14 + 20 + 8 + 17 payload
	bytesPerUDPPacketIPv6  = 79  // 14 + 40 + 8 + 17
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
// flow-report predicates and rules for a given (family, protocol)
// combination.
type probeProtocolParams struct {
	// name is the value passed to --protocol on the tester scenarios.
	name string
	// ipProto is the IP protocol number reported by flow records and
	// matched by SDF rules.
	ipProto uint8
	// flowsPerUE is the number of distinct (5-tuple) flows the probe
	// produces per UE per direction. ICMP echo and UDP echo collapse
	// to one flow each; TCP opens probeRoundtrips connections.
	flowsPerUE int
	// packetsPerFlow is the count we expect each emitted flow to
	// carry, when known. Nil for TCP, whose count is kernel-dependent.
	packetsPerFlow *uint64
	// bytesPerFlow is the count we expect each emitted flow to carry.
	// Nil when not deterministic (TCP).
	bytesPerFlow *uint64
	// supportsPortRules is true for protocols whose SDF match
	// includes L4 ports (TCP, UDP).
	supportsPortRules bool
}

// familyParams picks the parameter set matching the active IP family.
// DualStack reuses the IPv4 leg, matching the convention used
// elsewhere in the integration suite.
func familyParams(family IPFamily) ipFamilyParams {
	if family == IPv6Only {
		return ipFamilyParams{
			family:            IPv6Only,
			scenarioAllowed:   "ue/connectivity_expect_allowed_ipv6",
			scenarioBlocked:   "ue/connectivity_expect_blocked_ipv6",
			pingDestination:   scenarios.DefaultPingDestinationV6,
			uePool:            scenarios.DefaultUEIPv6Pool,
			nonMatchingPrefix: "2001:db8:dead::/48",
			hostPrefix:        "/128",
		}
	}

	return ipFamilyParams{
		family:            family,
		scenarioAllowed:   "ue/connectivity_expect_allowed",
		scenarioBlocked:   "ue/connectivity_expect_blocked",
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
			packetsPerFlow:    nil, // calibrated from first run
			bytesPerFlow:      nil, // calibrated from first run
			supportsPortRules: true,
		}
	case "udp":
		bytes := bytesPerUDPPacketIPv4
		if family == IPv6Only {
			bytes = bytesPerUDPPacketIPv6
		}

		packets := uint64(probeRoundtrips)
		bytesPerFlow := uint64(probeRoundtrips) * uint64(bytes)

		return probeProtocolParams{
			name:              "udp",
			ipProto:           ipProtoUDP,
			flowsPerUE:        1,
			packetsPerFlow:    &packets,
			bytesPerFlow:      &bytesPerFlow,
			supportsPortRules: true,
		}
	default: // icmp
		ipProto := ipProtoICMP
		bytes := bytesPerICMPPacketIPv4

		if family == IPv6Only {
			ipProto = ipProtoICMPv6
			bytes = bytesPerICMPPacketIPv6
		}

		packets := uint64(probeRoundtrips)
		bytesPerFlow := uint64(probeRoundtrips) * uint64(bytes)

		return probeProtocolParams{
			name:              "icmp",
			ipProto:           ipProto,
			flowsPerUE:        1,
			packetsPerFlow:    &packets,
			bytesPerFlow:      &bytesPerFlow,
			supportsPortRules: false,
		}
	}
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
// for the polling loop. Each-prefixed predicates short-circuit on an
// empty snapshot to avoid vacuous-truth matches.
func expectedFlowsContentPredicate(direction, action string, expectedIMSIs []string, fp ipFamilyParams, pp probeProtocolParams) fixture.FlowReportPredicate {
	expandedIMSIs := repeatIMSIs(expectedIMSIs, pp.flowsPerUE)

	preds := []fixture.FlowReportPredicate{
		fixture.Count(len(expandedIMSIs)),
		fixture.EachAction(action),
		fixture.EachDirection(direction),
		fixture.EachProtocol(pp.ipProto),
		fixture.ImsisAre(expandedIMSIs),
	}

	if pp.packetsPerFlow != nil {
		preds = append(preds, fixture.EachPackets(*pp.packetsPerFlow))
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

	return fixture.And(preds...)
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
func scenarioRunArgs(name string, spec scenarios.FixtureSpec, pp probeProtocolParams) []string {
	var args []string

	if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok {
		args = append(args, "--ip-version", string(requiredFamily))
	}

	args = append(args, "--protocol", pp.name)
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
