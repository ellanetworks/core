// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "net/netip"

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
}

func GBRQCI(qci uint8) bool {
	return (qci >= 1 && qci <= 4) || (qci >= 65 && qci <= 67) || (qci >= 75 && qci <= 76) || (qci >= 82 && qci <= 85)
}
