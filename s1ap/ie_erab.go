// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"net/netip"

	"github.com/ellanetworks/core/per"
)

// ERABID ::= INTEGER (0..15, ...) (extensible).
type ERABID uint8

func (id ERABID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 15, int64(id))
}

func (id *ERABID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 15, "E-RAB-ID")
	if err != nil {
		return err
	}

	*id = ERABID(v)

	return nil
}

// QCI ::= INTEGER (0..255).
type QCI uint8

func (q QCI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(q))
}

func (q *QCI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*q = QCI(v)

	return nil
}

// Pre-emptionCapability ::= ENUMERATED (not extensible).
type PreemptionCapability uint8

const (
	PreemptionShallNotTrigger PreemptionCapability = iota
	PreemptionMayTrigger
)

// Pre-emptionVulnerability ::= ENUMERATED (not extensible).
type PreemptionVulnerability uint8

const (
	PreemptionNotPreemptable PreemptionVulnerability = iota
	PreemptionPreemptable
)

// AllocationAndRetentionPriority ::= SEQUENCE { priorityLevel,
// pre-emptionCapability, pre-emptionVulnerability, iE-Extensions OPTIONAL }
// (extensible). PriorityLevel ::= INTEGER (0..15).
type AllocationAndRetentionPriority struct {
	PriorityLevel           uint8                   `per:",range:0..15"`
	PreemptionCapability    PreemptionCapability    `per:"ENUMERATED,range:0..1"`
	PreemptionVulnerability PreemptionVulnerability `per:"ENUMERATED,range:0..1"`
	iEExtensions            *rawIEExtensions        `per:",optional"`
	extensions              per.ExtensionAdditions  `per:",extraw"`
}

// GBRQosInformation ::= SEQUENCE { four BitRates, iE-Extensions OPTIONAL }
// (extensible).
type GBRQosInformation struct {
	MaximumBitrateDL            BitRate
	MaximumBitrateUL            BitRate
	GuaranteedBitrateDL         BitRate
	GuaranteedBitrateUL         BitRate
	ExtendedMaximumBitrateDL    *ExtendedBitRate       `per:"-"`
	ExtendedMaximumBitrateUL    *ExtendedBitRate       `per:"-"`
	ExtendedGuaranteedBitrateDL *ExtendedBitRate       `per:"-"`
	ExtendedGuaranteedBitrateUL *ExtendedBitRate       `per:"-"`
	iEExtensions                messageMeta            `per:",exttable:gBRQosInformationExtIEs"`
	extensions                  per.ExtensionAdditions `per:",extraw"`
}

var gBRQosInformationExtIEs = []ieSpec[GBRQosInformation]{
	optionalExtension(IDExtendedERABMaximumBitrateDL, CriticalityIgnore, func(m *GBRQosInformation) **ExtendedBitRate {
		return &m.ExtendedMaximumBitrateDL
	}),
	optionalExtension(IDExtendedERABMaximumBitrateUL, CriticalityIgnore, func(m *GBRQosInformation) **ExtendedBitRate {
		return &m.ExtendedMaximumBitrateUL
	}),
	optionalExtension(IDExtendedERABGuaranteedBitrateDL, CriticalityIgnore, func(m *GBRQosInformation) **ExtendedBitRate {
		return &m.ExtendedGuaranteedBitrateDL
	}),
	optionalExtension(IDExtendedERABGuaranteedBitrateUL, CriticalityIgnore, func(m *GBRQosInformation) **ExtendedBitRate {
		return &m.ExtendedGuaranteedBitrateUL
	}),
}

// ERABLevelQoSParameters ::= SEQUENCE { qCI, allocationRetentionPriority,
// gbrQosInformation OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type ERABLevelQoSParameters struct {
	QCI                    QCI
	ARP                    AllocationAndRetentionPriority
	GBR                    *GBRQosInformation     `per:",optional"`
	DownlinkPacketLossRate *PacketLossRate        `per:"-"`
	UplinkPacketLossRate   *PacketLossRate        `per:"-"`
	iEExtensions           messageMeta            `per:",exttable:eRABLevelQoSParametersExtIEs"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

var eRABLevelQoSParametersExtIEs = []ieSpec[ERABLevelQoSParameters]{
	optionalExtension(IDDownlinkPacketLossRate, CriticalityIgnore, func(m *ERABLevelQoSParameters) **PacketLossRate {
		return &m.DownlinkPacketLossRate
	}),
	optionalExtension(IDUplinkPacketLossRate, CriticalityIgnore, func(m *ERABLevelQoSParameters) **PacketLossRate {
		return &m.UplinkPacketLossRate
	}),
}

// TransportLayerAddress ::= BIT STRING (SIZE(1..160, ...)). Holds the address
// octets (IPv4 = 4, IPv6 = 16).
type TransportLayerAddress []byte

func (a TransportLayerAddress) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeBitString(w, enc, 1, 160, true, true, true, a, len(a)*8)
}

func (a *TransportLayerAddress) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, 1, 160, true, true, true)
	if err != nil {
		return err
	}

	*a = TransportLayerAddress(b[:(nbits+7)/8])

	return nil
}

// IPs returns the addresses the bit string carries. TS 36.414 §5.1 packs an
// IPv4 address, an IPv6 address, or both with the IPv4 one first; a return is
// invalid when that address is absent.
func (a TransportLayerAddress) IPs() (ipv4, ipv6 netip.Addr) {
	switch len(a) {
	case 4:
		ipv4, _ = netip.AddrFromSlice(a)
	case 16:
		ipv6, _ = netip.AddrFromSlice(a)
	case 20:
		ipv4, _ = netip.AddrFromSlice(a[:4])
		ipv6, _ = netip.AddrFromSlice(a[4:])
	}

	return ipv4, ipv6
}

func (a TransportLayerAddress) Valid() bool {
	ipv4, ipv6 := a.IPs()

	return ipv4.IsValid() || ipv6.IsValid()
}

// GTPTEID ::= OCTET STRING (SIZE(4)).
type GTPTEID uint32

func (t GTPTEID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, []byte{byte(t >> 24), byte(t >> 16), byte(t >> 8), byte(t)})
}

func (t *GTPTEID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*t = GTPTEID(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]))

	return nil
}

// ERABToBeSetupItemCtxtSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU OPTIONAL, iE-Extensions OPTIONAL }
// (extensible).
type ERABToBeSetupItemCtxtSUReq struct {
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU                 `per:",optional"`
	CorrelationID         *CorrelationID         `per:"-"`
	SIPTOCorrelationID    *CorrelationID         `per:"-"`
	BearerType            *BearerType            `per:"-"`
	EthernetType          *EthernetType          `per:"-"`
	SecurityIndication    *SecurityIndication    `per:"-"`
	iEExtensions          messageMeta            `per:",exttable:eRABToBeSetupItemCtxtSUReqExtIEs"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

var eRABToBeSetupItemCtxtSUReqExtIEs = []ieSpec[ERABToBeSetupItemCtxtSUReq]{
	optionalExtension(IDCorrelationID, CriticalityIgnore, func(m *ERABToBeSetupItemCtxtSUReq) **CorrelationID {
		return &m.CorrelationID
	}),
	optionalExtension(IDSIPTOCorrelationID, CriticalityIgnore, func(m *ERABToBeSetupItemCtxtSUReq) **CorrelationID {
		return &m.SIPTOCorrelationID
	}),
	optionalExtension(IDBearerType, CriticalityReject, func(m *ERABToBeSetupItemCtxtSUReq) **BearerType {
		return &m.BearerType
	}),
	optionalExtension(IDEthernetType, CriticalityIgnore, func(m *ERABToBeSetupItemCtxtSUReq) **EthernetType {
		return &m.EthernetType
	}),
	optionalExtension(IDSecurityIndication, CriticalityReject, func(m *ERABToBeSetupItemCtxtSUReq) **SecurityIndication {
		return &m.SecurityIndication
	}),
}

// ERABSetupItemCtxtSURes ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible).
type ERABSetupItemCtxtSURes struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	iEExtensions          *rawIEExtensions       `per:",optional"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// ERABItem ::= SEQUENCE { e-RAB-ID, cause, iE-Extensions OPTIONAL } (extensible).
type ERABItem struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	Cause  Cause
	_      ieExtensions `per:",skip"`
}

type BearerType uint8

const (
	BearerTypeNonIP BearerType = iota

	bearerTypeRootCount = 1
)

func (b BearerType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, bearerTypeRootCount, int64(b), "BearerType")
}

func (b *BearerType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, bearerTypeRootCount, "BearerType")
	if err != nil {
		return err
	}

	*b = BearerType(idx)

	return nil
}

type EthernetType uint8

const (
	EthernetTypeTrue EthernetType = iota

	ethernetTypeRootCount = 1
)

func (e EthernetType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, ethernetTypeRootCount, int64(e), "Ethernet-Type")
}

func (e *EthernetType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, ethernetTypeRootCount, "Ethernet-Type")
	if err != nil {
		return err
	}

	*e = EthernetType(idx)

	return nil
}

type IntegrityProtectionIndication uint8

const (
	IntegrityProtectionRequired IntegrityProtectionIndication = iota
	IntegrityProtectionPreferred
	IntegrityProtectionNotNeeded

	integrityProtectionIndicationRootCount = 3
)

func (i IntegrityProtectionIndication) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, integrityProtectionIndicationRootCount, int64(i), "IntegrityProtectionIndication")
}

func (i *IntegrityProtectionIndication) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, integrityProtectionIndicationRootCount, "IntegrityProtectionIndication")
	if err != nil {
		return err
	}

	*i = IntegrityProtectionIndication(idx)

	return nil
}

type SecurityIndication struct {
	IntegrityProtectionIndication IntegrityProtectionIndication
	iEExtensions                  *rawIEExtensions       `per:",optional"`
	extensions                    per.ExtensionAdditions `per:",extraw"`
}

type CorrelationID [4]byte

func (c CorrelationID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, c[:])
}

func (c *CorrelationID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	copy(c[:], b)

	return nil
}

type PacketLossRate uint16

func (p PacketLossRate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeConstrainedWholeNumber(w, enc, 0, 1000, int64(p))
}

func (p *PacketLossRate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 1000)
	if err != nil {
		return err
	}

	*p = PacketLossRate(v)

	return nil
}

const (
	extendedBitRateMin = 10000000001
	extendedBitRateMax = 4000000000000
)

type ExtendedBitRate uint64

func (b ExtendedBitRate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, extendedBitRateMin, extendedBitRateMax, int64(b))
}

func (b *ExtendedBitRate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, extendedBitRateMin, extendedBitRateMax, "ExtendedBitRate")
	if err != nil {
		return err
	}

	*b = ExtendedBitRate(v)

	return nil
}
