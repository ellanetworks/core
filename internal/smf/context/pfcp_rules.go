// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"
	"time"
)

const (
	RULE_INITIAL RuleState = 0
	RULE_CREATE  RuleState = 1
	RULE_UPDATE  RuleState = 2
	RULE_REMOVE  RuleState = 3
)

type RuleState uint8

// Packet Detection Rule. Table 7.5.2.2-1
type PDR struct {
	OuterHeaderRemoval *OuterHeaderRemoval

	FAR *FAR
	URR *URR
	QER []*QER

	PDI        PDI
	State      RuleState
	PDRID      uint16
	Precedence uint32
}

type SDFFilter struct {
	FlowDescription         []byte
	TosTrafficClass         []byte
	SecurityParameterIndex  []byte
	FlowLabel               []byte
	SdfFilterId             uint32
	LengthOfFlowDescription uint16
	Bid                     bool
	Fl                      bool
	Spi                     bool
	Ttc                     bool
	Fd                      bool
}

type FTEID struct {
	Ipv4Address net.IP
	Ipv6Address net.IP
	Chid        bool
	Ch          bool
	V6          bool
	V4          bool
	Teid        uint32
	ChooseId    uint8
}

type UEIPAddress struct {
	Ipv4Address              net.IP
	Ipv6Address              net.IP
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
	LocalFTeid      *FTEID
	UEIPAddress     *UEIPAddress
	SDFFilter       *SDFFilter
	ApplicationID   string
	NetworkInstance string
	SourceInterface SourceInterface
}

// Forwarding Action Rule. 7.5.2.3-1
type FAR struct {
	ForwardingParameters *ForwardingParameters

	BAR   *BAR
	State RuleState
	FARID uint32

	ApplyAction ApplyAction
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
	BARID uint8

	DownlinkDataNotificationDelay  DownlinkDataNotificationDelay
	SuggestedBufferingPacketsCount SuggestedBufferingPacketsCount

	State RuleState
}

// QoS Enhancement Rule
type QER struct {
	GateStatus *GateStatus
	MBR        *MBR
	GBR        *GBR

	State RuleState
	QFI   QFI
	QERID uint32
}

// Usage Report Rule
type URR struct{}

func (pdr PDR) String() string {
	return fmt.Sprintf("PDR:[PdrId:[%v], Precedence:[%v], PDI:[%v], OuterHeaderRem:[%v], Far:[%v], RuleState:[%v], QERS:[%v]]",
		pdr.PDRID, pdr.Precedence, pdr.PDI, pdr.OuterHeaderRemoval, pdr.FAR, pdr.State, pdr.QER)
}

func (pdi PDI) String() string {
	return fmt.Sprintf("PDI:[SourceInterface:[%v], LocalFteid:[%v], NetworkInstance:[%v], UEIpAddr:[%v], SdfFilter:[%v], AppId:[%v]]",
		pdi.SourceInterface, pdi.LocalFTeid, pdi.NetworkInstance, pdi.UEIPAddress, pdi.SDFFilter, pdi.ApplicationID)
}

func (far FAR) String() string {
	return fmt.Sprintf("FAR:[Id:[%v], ApplyAction:[%v], FrwdParam:[%v], BAR:[%v], State:[%v]]",
		far.FARID, ActionString(far.ApplyAction), far.ForwardingParameters, far.BAR, far.State)
}

func ActionString(act ApplyAction) string {
	return fmt.Sprintf("Action:[Dup:%v, Nocp:%v, Buff:%v, Forw:%v, Drop:%v]", act.Dupl, act.Nocp, act.Buff, act.Forw, act.Drop)
}

func (fp ForwardingParameters) String() string {
	return fmt.Sprintf("FwdParam:[DestIntf:[%v], NetworkInstance:[%v], OuterHeaderCreation:[%v], PFCPSMReqFlags:[%v], ForwardingPolicyID:[%v]]",
		fp.DestinationInterface, fp.NetworkInstance, fp.OuterHeaderCreation, fp.PFCPSMReqFlags, fp.ForwardingPolicyID)
}

func (bar BAR) String() string {
	return fmt.Sprintf("\nBAR:[Id:[%v], DDNDelay:[%v], BuffPktCount:[%v], RuleState:[%v]]",
		bar.BARID, bar.DownlinkDataNotificationDelay.DelayValue, bar.SuggestedBufferingPacketsCount.PacketCountValue, bar.State)
}

func (qer QER) String() string {
	return fmt.Sprintf("\nQER:[Id:[%v], QFI:[%v], MBR:[UL:[%v], DL:[%v]], Gate:[UL:[%v], DL:[%v]], RuleState:[%v]]",
		qer.QERID, qer.QFI, qer.MBR.ULMBR, qer.MBR.DLMBR, qer.GateStatus.ULGate, qer.GateStatus.DLGate, qer.State)
	// return fmt.Sprintf("\nQER:[Id:[%v], QFI:[%v], MBR:[UL:[%v], DL:[%v]], GBR:[UL:[%v], DL:[%v]], Gate:[UL:[%v], DL:[%v]], RuleState:[%v]] ",
	//	qer.QERID, qer.QFI, qer.MBR.ULMBR, qer.MBR.DLMBR, qer.GBR.ULGBR, qer.GBR.DLGBR, qer.GateStatus.ULGate, qer.GateStatus.DLGate, qer.State)
}
