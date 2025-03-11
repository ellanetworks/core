// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"net"
	"time"
)

const (
	RuleInitial RuleState = 0
	RuleCreate  RuleState = 1
	RuleUpdate  RuleState = 2
	RuleRemove  RuleState = 3
)

type RuleState uint8

// Packet Detection Rule. Table 7.5.2.2-1
type PDR struct {
	OuterHeaderRemoval *OuterHeaderRemoval
	FAR                *FAR
	QER                []*QER
	PDI                PDI
	State              RuleState
	PDRID              uint16
	Precedence         uint32
}

type SDFFilter struct {
	FlowDescription         []byte
	TosTrafficClass         []byte
	SecurityParameterIndex  []byte
	FlowLabel               []byte
	SdfFilterID             uint32
	LengthOfFlowDescription uint16
	Bid                     bool
	Fl                      bool
	Spi                     bool
	Ttc                     bool
	Fd                      bool
}

type FTEID struct {
	IPv4Address net.IP
	IPv6Address net.IP
	Chid        bool
	Ch          bool
	V6          bool
	V4          bool
	TeID        uint32
	ChooseID    uint8
}

type UEIPAddress struct {
	IPv4Address              net.IP
	IPv6Address              net.IP
	V6                       bool // bit 1
	V4                       bool // bit 2
	Sd                       bool // bit 3
	Ipv6d                    bool // bit 4
	CHV4                     bool // bit 5
	CHV6                     bool // bit 6
	IP6PL                    bool // bit 7
	Ipv6PrefixDelegationBits uint8
	Ipv6PrefixLength         uint8
}

// Packet Detection. 7.5.2.2-2
type PDI struct {
	LocalFTeID      *FTEID
	UEIPAddress     *UEIPAddress
	SDFFilter       *SDFFilter
	ApplicationID   string
	NetworkInstance string
	SourceInterface SourceInterface
}

// Forwarding Action Rule. 7.5.2.3-1
type FAR struct {
	ForwardingParameters *ForwardingParameters
	BAR                  *BAR
	State                RuleState
	FARID                uint32
	ApplyAction          ApplyAction
}

type PFCPSMReqFlags struct {
	Qaurr bool
	Sndem bool
	Drobu bool
}

// Forwarding Parameters. 7.5.2.3-2
type ForwardingParameters struct {
	OuterHeaderCreation  *OuterHeaderCreation
	PFCPSMReqFlags       *PFCPSMReqFlags
	ForwardingPolicyID   string
	NetworkInstance      string
	DestinationInterface DestinationInterface
}

type SuggestedBufferingPacketsCount struct {
	PacketCountValue uint8
}

type DownlinkDataNotificationDelay struct {
	DelayValue time.Duration
}

// Buffering Action Rule 7.5.2.6-1
type BAR struct {
	BARID                          uint8
	DownlinkDataNotificationDelay  DownlinkDataNotificationDelay
	SuggestedBufferingPacketsCount SuggestedBufferingPacketsCount
	State                          RuleState
}

// QoS Enhancement Rule
type QER struct {
	GateStatus *GateStatus
	MBR        *MBR
	GBR        *GBR
	State      RuleState
	QFI        QFI
	QERID      uint32
}
