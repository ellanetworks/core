package scenarios

import (
	"net"
	"os"
)

// IPFamily represents the IP address family mode for the test environment.
type IPFamily int

const (
	IPv4Only IPFamily = iota
	IPv6Only
	DualStack
)

func detectIPFamily() IPFamily {
	switch os.Getenv("IP_VERSION") {
	case "v6", "ipv6":
		return IPv6Only
	case "dual", "dualstack", "both":
		return DualStack
	default:
		return IPv4Only
	}
}

// Env carries the common flag values to scenario runners.
//
// Populated by cmd/core-tester from the common flag families
// (--ella-core-n2-address, --gnb, --gnb-core-target).
type Env struct {
	// CoreN2Addresses are every value supplied via --ella-core-n2-address,
	// in argument order. Single-core scenarios consume the first entry.
	CoreN2Addresses []string

	// GNBs lists every gNB declared via --gnb, in argument order.
	GNBs []GNB

	// GNBCoreTargets maps gNB name → core N2 address for scenarios that
	// need explicit pairing. When empty, scenarios default to pairing gNB
	// i with CoreN2Addresses[i], or all cores for multihomed scenarios.
	GNBCoreTargets map[string]string
}

// GNB is one simulated gNB's address set.
type GNB struct {
	Name        string
	N2Address   string
	N3Address   string
	N3Secondary string
}

// HasIPv6 returns true when the gNB N3 address is an IPv6 address.
func (g GNB) HasIPv6() bool {
	ip := net.ParseIP(g.N3Address)
	return ip != nil && ip.To4() == nil
}

// FirstCore returns CoreN2Addresses[0], or "" when empty.
func (e Env) FirstCore() string {
	if len(e.CoreN2Addresses) == 0 {
		return ""
	}

	return e.CoreN2Addresses[0]
}

// FirstGNB returns GNBs[0], or a zero GNB when empty.
func (e Env) FirstGNB() GNB {
	if len(e.GNBs) == 0 {
		return GNB{}
	}

	return e.GNBs[0]
}

// IPFamily returns the IP address family configured for the test environment.
func (e Env) IPFamily() IPFamily {
	return detectIPFamily()
}

// HasIPv4 returns true when the test environment supports IPv4.
func (e Env) HasIPv4() bool {
	family := e.IPFamily()
	return family == IPv4Only || family == DualStack
}

// HasIPv6 returns true when the test environment supports IPv6.
func (e Env) HasIPv6() bool {
	family := e.IPFamily()
	return family == IPv6Only || family == DualStack
}

// PingDestination returns the appropriate ping destination address for the
// current IP family. In IPv4-only mode it returns the IPv4 address, in
// IPv6-only mode it returns the IPv6 address, and in dual-stack mode it
// returns the IPv4 address for backward compatibility.
func (e Env) PingDestination() string {
	family := e.IPFamily()
	switch family {
	case IPv6Only:
		return DefaultPingDestinationV6
	default:
		return DefaultPingDestination
	}
}

// PingDestinationV6 returns the IPv6 ping destination address.
// Returns empty string when IPv6 is not available.
func (e Env) PingDestinationV6() string {
	if e.HasIPv6() {
		return DefaultPingDestinationV6
	}

	return ""
}

// PDUSessionType returns the appropriate PDU Session Type for the current IP
// family. In IPv4-only mode it returns IPv4, in IPv6-only mode it returns
// IPv6, and in dual-stack mode it returns IPv4+IPv6.
func (e Env) PDUSessionType() uint8 {
	family := e.IPFamily()
	switch family {
	case IPv6Only:
		return DefaultPDUSessionTypeIPv6
	case DualStack:
		return DefaultPDUSessionTypeIPv4IPv6
	default:
		return DefaultPDUSessionTypeIPv4
	}
}

// UIPrefix returns the appropriate IP prefix length for the current IP family.
// Returns "/16" for IPv4 and "/64" for IPv6.
func (e Env) UIPrefix() string {
	family := e.IPFamily()
	switch family {
	case IPv6Only, DualStack:
		return "/64"
	default:
		return "/16"
	}
}

// PingCommand returns the appropriate ping command for the current IP family.
// Returns "ping" for IPv4 and "ping6" for IPv6.
func (e Env) PingCommand() string {
	family := e.IPFamily()
	switch family {
	case IPv6Only, DualStack:
		return "ping6"
	default:
		return "ping"
	}
}
