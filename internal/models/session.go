// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"net"
	"net/netip"
)

// EstablishRequest asks the UPF to create a new session with the
// given packet detection, forwarding, QoS, and usage reporting rules.
type EstablishRequest struct {
	LocalSEID uint64
	IMSI      string
	PolicyID  string
	PDRs      []PDR
	FARs      []FAR
	QERs      []QER
	URRs      []URR
}

// EstablishResponse returns the allocated identifiers from the UPF.
type EstablishResponse struct {
	RemoteSEID  uint64
	CreatedPDRs []CreatedPDR
}

// CreatedPDR describes a PDR created by the UPF with any allocated
// resources (TEID for GTP uplink, or UE IP confirmation for downlink).
type CreatedPDR struct {
	PDRID  uint16
	TEID   uint32
	N3IPv4 netip.Addr // may be zero if not available
	N3IPv6 netip.Addr // may be zero if not available
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

// FTEID is a fully qualified Tunnel Endpoint Identifier (TS 29.244 §8.2.3): a
// GTP-U TEID together with the IP address that terminates the tunnel. A zero
// value (TEID 0, invalid Addr) signals "to be assigned by the UPF" when used as
// a PDI local F-TEID.
type FTEID struct {
	TEID uint32
	Addr netip.Addr
}

// EPSBearerRequest is the input the MME hands the SMF+PGW-C anchor to establish a
// 4G default bearer. IPv4Pool/IPv6Pool are non-empty when the data network offers
// that family; RequestedPDNType is the UE's requested type (1 IPv4, 2 IPv6,
// 3 IPv4v6, TS 24.301 §9.9.4.10).
type EPSBearerRequest struct {
	IMSI string
	// EPSBearerIdentity is the default bearer's EBI (5..15), which the SMF uses as
	// the PDU session id keying this PDN connection so one IMSI can hold several.
	EPSBearerIdentity uint8
	PolicyID          string // policy DB ID, so the UPF binds the session to its network rules
	APN               string
	AMBRUplink        string
	AMBRDownlink      string
	IPv4Pool          string
	IPv6Pool          string
	DNS               string
	MTU               uint16
	RequestedPDNType  uint8
}

// EPSBearer is the result of establishing a default bearer: the negotiated PDN
// type, the allocated addresses (the IPv6 /64 prefix + the SLAAC interface
// identifier; the prefix is delivered to the UE via Router Advertisement), and
// the S-GW S1-U F-TEID the eNB sends uplink traffic to.
type EPSBearer struct {
	PDNType    uint8
	IPv4       netip.Addr
	IPv6Prefix netip.Addr
	IPv6IID    [8]byte
	DNS        netip.Addr
	// SGW.Addr is the IPv4 S1-U N3 endpoint (invalid on an IPv6-only N3); SGWN3IPv6
	// is the IPv6 one. The MME advertises whichever the N3 has to the eNB.
	SGW       FTEID
	SGWN3IPv6 netip.Addr
	// ESMCause, when non-zero, is the reason the assigned PDN type is narrower
	// than requested (#50 IPv4-only / #51 IPv6-only allowed, TS 24.301 §6.5.1.3).
	ESMCause uint8
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
	IPv6Address net.IP // new field for IPv6 transport

	// S1U marks a 4G S1-U tunnel, whose GTP-U G-PDUs carry no PDU Session
	// Container / QFI extension header (that is N3/N9-only, TS 38.415); the
	// datapath emits a plain G-PDU when set. Default (false) keeps the 5G N3
	// behaviour unchanged.
	S1U bool
}

// PFCP outer header constants.
const (
	OuterHeaderCreationGtpUUdpIpv4 uint16 = 256
	OuterHeaderCreationGtpUUdpIpv6 uint16 = 512
	OuterHeaderRemovalGtpUUdpIpv4  uint8  = 0
	OuterHeaderRemovalGtpUUdpIpv6  uint8  = 1
)

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

// PFCP gate status values.
const (
	GateOpen  uint8 = 0
	GateClose uint8 = 1
)

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
	SEID     uint64
	PolicyID string

	CreatePDRs   []PDR
	UpdatePDRs   []PDR
	RemovePDRIDs []uint16

	CreateFARs   []FAR
	UpdateFARs   []FAR
	RemoveFARIDs []uint32

	CreateQERs   []QER
	UpdateQERs   []QER
	RemoveQERIDs []uint32
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

// IPv6SessionRegistration carries the metadata the SMF provides to the UPF
// so the RA responder can reply to Router Solicitations from IPv6 UEs.
type IPv6SessionRegistration struct {
	UplinkTEID   uint32       // UL TEID allocated by the UPF
	DownlinkTEID uint32       // DL TEID provided by the gNB
	GnbN3Addr    netip.Addr   // gNB's N3 transport address (IPv4 or IPv6)
	Prefix       netip.Prefix // delegated /64 prefix
	MTU          uint32       // DNN MTU
	QFI          uint8        // QoS Flow Identifier
	S1U          bool         // 4G S1-U: encapsulate the RA PSC-less (no PDU Session Container)
}
