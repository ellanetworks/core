// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"net"
	"os"
)

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

// Env carries the common flag values to scenario runners, populated by
// cmd/core-tester from the --ella-core-n2-address, --gnb, and
// --gnb-core-target flag families.
type Env struct {
	// CoreN2Addresses holds every --ella-core-n2-address in argument order;
	// single-core scenarios consume the first entry.
	CoreN2Addresses []string

	GNBs []GNB

	// GNBCoreTargets pairs gNB name → core N2 address. When empty, scenarios
	// pair gNB i with CoreN2Addresses[i], or all cores for multihomed ones.
	GNBCoreTargets map[string]string

	APIAddress string
	APIToken   string
}

type GNB struct {
	Name        string
	N2Address   string
	N3Address   string
	N3Secondary string
}

func (g GNB) HasIPv6() bool {
	ip := net.ParseIP(g.N3Address)
	return ip != nil && ip.To4() == nil
}

func (e Env) FirstCore() string {
	if len(e.CoreN2Addresses) == 0 {
		return ""
	}

	return e.CoreN2Addresses[0]
}

func (e Env) FirstGNB() GNB {
	if len(e.GNBs) == 0 {
		return GNB{}
	}

	return e.GNBs[0]
}

func (e Env) IPFamily() IPFamily {
	return detectIPFamily()
}

func (e Env) HasIPv4() bool {
	family := e.IPFamily()
	return family == IPv4Only || family == DualStack
}

func (e Env) HasIPv6() bool {
	family := e.IPFamily()
	return family == IPv6Only || family == DualStack
}

// PingDestination returns the ping destination for the current IP family,
// preferring IPv4 in dual-stack mode.
func (e Env) PingDestination() string {
	family := e.IPFamily()
	switch family {
	case IPv6Only:
		return DefaultPingDestinationV6
	default:
		return DefaultPingDestination
	}
}

// PingDestinationV6 returns the IPv6 ping destination, or "" when IPv6 is
// unavailable.
func (e Env) PingDestinationV6() string {
	if e.HasIPv6() {
		return DefaultPingDestinationV6
	}

	return ""
}

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

func (e Env) UIPrefix() string {
	family := e.IPFamily()
	switch family {
	case IPv6Only, DualStack:
		return "/64"
	default:
		return "/16"
	}
}

func (e Env) PingCommand() string {
	family := e.IPFamily()
	switch family {
	case IPv6Only, DualStack:
		return "ping6"
	default:
		return "ping"
	}
}
