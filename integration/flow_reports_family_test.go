package integration_test

import (
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	expectedPacketsPerFlow = 3

	// 14 (Ethernet) + 20 (IPv4) + 8 (ICMP) + 56 (payload). Symmetric
	// between directions: the uplink path strips GTP/UDP/IP and
	// rewrites L2 before flow accounting runs.
	bytesPerPacketIPv4 = 98

	// 14 (Ethernet) + 40 (IPv6) + 8 (ICMPv6) + 56 (payload).
	bytesPerPacketIPv6 = 118

	// Tolerance covering clock skew between the kernel-derived flow
	// timestamps and the test's wall clock.
	timestampUpperBuffer = 5 * time.Second
)

// ipFamilyParams holds the per-family values needed to drive the
// flow-report tests against either an IPv4 or IPv6 PDU session.
type ipFamilyParams struct {
	family            IPFamily
	scenarioAllowed   string
	scenarioBlocked   string
	pingDestination   string
	uePool            string
	nonMatchingPrefix string
	hostPrefix        string // "/32" for IPv4, "/128" for IPv6
	protocolFilter    string // "1" (ICMP) or "58" (ICMPv6)
	ruleProtocol      int32  // 1 or 58
	bytesPerPacket    uint64
}

// familyParams picks the parameter set matching the active IP family. In
// DualStack mode we exercise the IPv4 leg of the dualstack session, as
// per the convention used elsewhere in the integration suite.
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
			protocolFilter:    "58",
			ruleProtocol:      58,
			bytesPerPacket:    bytesPerPacketIPv6,
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
		protocolFilter:    "1",
		ruleProtocol:      1,
		bytesPerPacket:    bytesPerPacketIPv4,
	}
}

// scenarioRunArgs mirrors the pattern TestIntegrationTester uses to build
// extra CLI args for core-tester: --ip-version is only injected for
// scenarios that are explicitly family-restricted, followed by any
// fixture-supplied extras.
func scenarioRunArgs(name string, spec scenarios.FixtureSpec) []string {
	var args []string

	if requiredFamily, ok := scenarioIPFamilyRestrictions[name]; ok {
		args = append(args, "--ip-version", string(requiredFamily))
	}

	args = append(args, spec.ExtraArgs...)

	return args
}

// apiSourceIPFilter returns the value to pass as the flow-report Source
// query parameter for the given direction. On IPv6 this excludes RA/RS
// background traffic, which has different source addresses than the
// ping target.
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

func expectedBytesPerFlow(fp ipFamilyParams) uint64 {
	return uint64(expectedPacketsPerFlow) * fp.bytesPerPacket
}

func expectedFlowsContentPredicate(direction, action string, expectedIMSIs []string, fp ipFamilyParams) fixture.FlowReportPredicate {
	preds := []fixture.FlowReportPredicate{
		fixture.Count(len(expectedIMSIs)),
		fixture.EachAction(action),
		fixture.EachDirection(direction),
		fixture.EachProtocol(uint8(fp.ruleProtocol)),
		fixture.EachPackets(expectedPacketsPerFlow),
		fixture.ImsisAre(expectedIMSIs),
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
