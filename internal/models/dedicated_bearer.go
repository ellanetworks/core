// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"net/netip"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type FilterDirection uint8

const (
	FilterDownlink      FilterDirection = 1
	FilterUplink        FilterDirection = 2
	FilterBidirectional FilterDirection = 3
)

type SDFFilter struct {
	ID         uint8
	Direction  FilterDirection
	Precedence uint8
	Protocol   uint8
	Remote     netip.Prefix
	LocalPort  uint16
	RemotePort uint16
}

type DedicatedBearerRequest struct {
	SessionRef string
	LinkedEBI  uint8
	QCI        uint8
	ARP        Arp
	MBR        Ambr
	GBR        Ambr
	Filters    []SDFFilter
	SGW        FTEID
	SGWN3IPv6  netip.Addr

	MappedFiveGSQoS []nas.PCOContainer
}

type DedicatedBearerContext struct {
	EBI       uint8
	QCI       uint8
	ARP       Arp
	MBR       Ambr
	GBR       Ambr
	Filters   []SDFFilter
	SGW       FTEID
	SGWN3IPv6 netip.Addr
}

type TFTOperation uint8

const (
	TFTNoChange TFTOperation = iota
	TFTAddFilters
	TFTReplaceFilters
	TFTDeleteFilters
)

type DedicatedBearerModification struct {
	SessionRef string
	SGWTEID    uint32
	QoSChanged bool
	MBR        Ambr
	GBR        Ambr
	ARP        *Arp
	Operation  TFTOperation
	Filters    []SDFFilter
	DeleteIDs  []uint8

	MappedFiveGSQoS []nas.PCOContainer
}

func GBRQCI(qci uint8) bool {
	return (qci >= 1 && qci <= 4) || (qci >= 65 && qci <= 67) || (qci >= 75 && qci <= 76) || (qci >= 82 && qci <= 85)
}

func EPSTFT(op eps.TFTOperation, filters []SDFFilter) eps.TrafficFlowTemplate {
	t := eps.TrafficFlowTemplate{Operation: op}

	for _, f := range filters {
		var comps []eps.TFTComponent

		if c, err := eps.RemoteAddress(f.Remote); err == nil {
			comps = append(comps, c)
		}

		if f.Protocol != 0 {
			comps = append(comps, eps.ProtocolIdentifier(f.Protocol))
		}

		if f.LocalPort != 0 {
			comps = append(comps, eps.SingleLocalPort(f.LocalPort))
		}

		if f.RemotePort != 0 {
			comps = append(comps, eps.SingleRemotePort(f.RemotePort))
		}

		t.Filters = append(t.Filters, eps.TFTPacketFilter{
			Identifier: f.ID,
			Direction:  eps.TFTDirection(f.Direction),
			Precedence: f.Precedence,
			Components: comps,
		})
	}

	return t
}

type DedicatedBearerEndpoint struct {
	SGWTEID uint32
	ENB     FTEID
}
