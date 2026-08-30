// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"fmt"
	"os"
	"strings"
)

type IPFamily int

const (
	IPv4Only IPFamily = iota
	IPv6Only
	DualStack
)

func detectIPFamily() IPFamily {
	family, err := ParseIPFamily(os.Getenv("IP_VERSION"))
	if err != nil {
		return IPv4Only
	}

	return family
}

func ParseIPFamily(s string) (IPFamily, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "v4", "ipv4":
		return IPv4Only, nil
	case "v6", "ipv6":
		return IPv6Only, nil
	case "dual", "dualstack", "both":
		return DualStack, nil
	default:
		return IPv4Only, fmt.Errorf("unknown IP family %q: want ipv4, ipv6 or dualstack", s)
	}
}

type Env struct {
	CoreN2Addresses []string
	GNBs            []GNB
	GNBCoreTargets  map[string]string
	APIAddress      string
	APIToken        string
}

type GNB struct {
	Name        string
	N2Address   string
	N3Address   string
	N3Secondary string
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
