// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"math"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// ERABID ::= INTEGER (0..15, ...) (extensible).
type ERABID uint8

func (id ERABID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, erabIDRootMax, int64(id))
}

func (id *ERABID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, erabIDRootMax)
	if err != nil {
		return err
	}

	if v < 0 || v > math.MaxUint8 {
		return per.ErrOverflow
	}

	*id = ERABID(v)

	return nil
}

const erabIDRootMax = 15

func (id *ERABID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return *id > erabIDRootMax }

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
	PriorityLevel           uint8 `per:",range:0..15"`
	PreemptionCapability    PreemptionCapability
	PreemptionVulnerability PreemptionVulnerability
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

// GBRQosInformation ::= SEQUENCE { four BitRates, iE-Extensions OPTIONAL }
// (extensible).
type GBRQosInformation struct {
	MaximumBitrateDL            BitRate
	MaximumBitrateUL            BitRate
	GuaranteedBitrateDL         BitRate
	GuaranteedBitrateUL         BitRate
	ExtendedMaximumBitrateDL    *ExtendedBitRate       `per:",extie:IDExtendedERABMaximumBitrateDL,crit:ignore"`
	ExtendedMaximumBitrateUL    *ExtendedBitRate       `per:",extie:IDExtendedERABMaximumBitrateUL,crit:ignore"`
	ExtendedGuaranteedBitrateDL *ExtendedBitRate       `per:",extie:IDExtendedERABGuaranteedBitrateDL,crit:ignore"`
	ExtendedGuaranteedBitrateUL *ExtendedBitRate       `per:",extie:IDExtendedERABGuaranteedBitrateUL,crit:ignore"`
	iEExtensions                extensionMeta          `per:",exttable"`
	extensions                  per.ExtensionAdditions `per:",extraw"`
}

// ERABLevelQoSParameters ::= SEQUENCE { qCI, allocationRetentionPriority,
// gbrQosInformation OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type ERABLevelQoSParameters struct {
	QCI                    QCI
	ARP                    AllocationAndRetentionPriority
	GBR                    *GBRQosInformation     `per:",optional"`
	DownlinkPacketLossRate *PacketLossRate        `per:",extie:IDDownlinkPacketLossRate,crit:ignore"`
	UplinkPacketLossRate   *PacketLossRate        `per:",extie:IDUplinkPacketLossRate,crit:ignore"`
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type TransportLayerAddress = ap.TransportLayerAddress

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
	CorrelationID         *CorrelationID         `per:",extie:IDCorrelationID,crit:ignore"`
	SIPTOCorrelationID    *CorrelationID         `per:",extie:IDSIPTOCorrelationID,crit:ignore"`
	BearerType            *BearerType            `per:",extie:IDBearerType,crit:reject"`
	EthernetType          *EthernetType          `per:",extie:IDEthernetType,crit:ignore"`
	SecurityIndication    *SecurityIndication    `per:",extie:IDSecurityIndication,crit:reject"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// ERABSetupItemCtxtSURes ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible).
type ERABSetupItemCtxtSURes struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// ERABItem ::= SEQUENCE { e-RAB-ID, cause, iE-Extensions OPTIONAL } (extensible).
type ERABItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	ERABID       ERABID
	Cause        Cause
	iEExtensions extensionMeta `per:",exttable"`
}

type BearerType uint8

const (
	BearerTypeNonIP BearerType = iota
)

type EthernetType uint8

const (
	EthernetTypeTrue EthernetType = iota
)

type IntegrityProtectionIndication uint8

const (
	IntegrityProtectionRequired IntegrityProtectionIndication = iota
	IntegrityProtectionPreferred
	IntegrityProtectionNotNeeded
)

type SecurityIndication struct {
	IntegrityProtectionIndication IntegrityProtectionIndication
	iEExtensions                  extensionMeta          `per:",exttable"`
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
	v, err := decodeExtensibleInt(r, enc, extendedBitRateMin, extendedBitRateMax)
	if err != nil {
		return err
	}

	*b = ExtendedBitRate(v)

	return nil
}

func (b *ExtendedBitRate) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return *b < extendedBitRateMin || *b > extendedBitRateMax
}
