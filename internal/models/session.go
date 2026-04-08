// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"net"
	"net/netip"
)

// EstablishRequest asks the UPF to create a new session with the
// given packet detection, forwarding, QoS, and usage reporting rules.
type EstablishRequest struct {
	LocalSEID          uint64
	IMSI               string
	PolicyID           int64
	PDRs               []PDR
	FARs               []FAR
	QERs               []QER
	URRs               []URR
	FilterIndexByPDRID map[uint16]uint32
}

// EstablishResponse returns the allocated identifiers from the UPF.
type EstablishResponse struct {
	RemoteSEID  uint64
	CreatedPDRs []CreatedPDR
}

// CreatedPDR describes a PDR created by the UPF with any allocated
// resources (TEID for GTP uplink, or UE IP confirmation for downlink).
type CreatedPDR struct {
	PDRID uint16
	TEID  uint32
	N3IP  netip.Addr
}

// PDR describes a Packet Detection Rule for the UPF session API.
type PDR struct {
	PDRID              uint16
	OuterHeaderRemoval *uint8
	FARID              uint32
	QERID              uint32
	URRID              uint32
	PDI                PDI
}

// PDI describes the Packet Detection Information for a PDR.
type PDI struct {
	LocalFTEID  *FTEID
	UEIPAddress netip.Addr
}

// FTEID describes a fully qualified Tunnel Endpoint Identifier.
type FTEID struct {
	TEID uint32
}

// FAR describes a Forwarding Action Rule for the UPF session API.
type FAR struct {
	FARID                uint32
	ApplyAction          ApplyAction
	ForwardingParameters *ForwardingParameters
}

// ApplyAction specifies the action to apply to matched packets.
type ApplyAction struct {
	Drop bool
	Forw bool
	Buff bool
	Nocp bool
	Dupl bool
}

// ForwardingParameters describes how to forward matched packets.
type ForwardingParameters struct {
	OuterHeaderCreation *OuterHeaderCreation
}

// OuterHeaderCreation describes GTP-U encapsulation parameters.
type OuterHeaderCreation struct {
	Description uint16
	TEID        uint32
	IPv4Address net.IP
}

// QER describes a QoS Enforcement Rule for the UPF session API.
type QER struct {
	QERID      uint32
	QFI        uint8
	GateStatus *GateStatus
	MBR        *MBR
}

// GateStatus controls uplink/downlink gate open/close.
type GateStatus struct {
	ULGate uint8
	DLGate uint8
}

// MBR holds the Maximum Bit Rate in kbps.
type MBR struct {
	ULMBR uint64
	DLMBR uint64
}

// URR describes a Usage Reporting Rule for the UPF session API.
type URR struct {
	URRID uint32
}

// ModifyRequest asks the UPF to modify an existing session.
// Rules are split into separate Create/Update/Remove slices
// mirroring the PFCP state machine (RuleInitial→Create,
// RuleUpdate→Update, RuleRemove→Remove).
type ModifyRequest struct {
	SEID               uint64
	PolicyID           int64
	FilterIndexByPDRID map[uint16]uint32

	CreatePDRs   []PDR
	UpdatePDRs   []PDR
	RemovePDRIDs []uint16

	CreateFARs   []FAR
	UpdateFARs   []FAR
	RemoveFARIDs []uint32

	CreateQERs []QER
}

// DeleteRequest asks the UPF to delete a session by its SEID.
type DeleteRequest struct {
	SEID uint64
}

// DownlinkDataReport notifies the SMF that buffered downlink data
// arrived for a UE, triggering paging.
type DownlinkDataReport struct {
	SEID  uint64
	PDRID uint16
	QFI   uint8
}

// UsageReport delivers periodic volume measurements from UPF to SMF.
type UsageReport struct {
	SEID           uint64
	UplinkVolume   uint64
	DownlinkVolume uint64
}

// FlowReportRequest is sent by UPF to SMF with flow statistics.
type FlowReportRequest struct {
	IMSI            string
	SourceIP        string
	DestinationIP   string
	SourcePort      uint16
	DestinationPort uint16
	Protocol        uint8
	Packets         uint64
	Bytes           uint64
	StartTime       string // RFC3339
	EndTime         string // RFC3339
	Direction       Direction
	Action          Action
}
