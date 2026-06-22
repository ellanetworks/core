// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// BitRate ::= INTEGER (0..10000000000).
const bitRateMax = 10000000000

type BitRate uint64

func (b BitRate) encode(w *aper.Writer) error {
	return w.WriteConstrainedInt(int64(b), 0, bitRateMax)
}

func decodeBitRate(r *aper.Reader) (BitRate, error) {
	v, err := r.ReadConstrainedInt(0, bitRateMax)
	return BitRate(v), err
}

// UEAggregateMaximumBitRate ::= SEQUENCE { ...DL, ...UL, iE-Extensions OPTIONAL }
// (extensible).
type UEAggregateMaximumBitRate struct {
	DL BitRate
	UL BitRate
}

func (a UEAggregateMaximumBitRate) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := a.DL.encode(w); err != nil {
		return err
	}

	return a.UL.encode(w)
}

func decodeUEAggregateMaximumBitRate(r *aper.Reader) (UEAggregateMaximumBitRate, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return UEAggregateMaximumBitRate{}, err
	}

	dl, err := decodeBitRate(r)
	if err != nil {
		return UEAggregateMaximumBitRate{}, err
	}

	ul, err := decodeBitRate(r)
	if err != nil {
		return UEAggregateMaximumBitRate{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return UEAggregateMaximumBitRate{}, err
	}

	return UEAggregateMaximumBitRate{DL: dl, UL: ul}, nil
}

// ERABID ::= INTEGER (0..15, ...) (extensible).
type ERABID uint8

func (id ERABID) encode(w *aper.Writer) error {
	w.WriteBool(false) // root value, not an extension

	return w.WriteConstrainedInt(int64(id), 0, 15)
}

func decodeERABID(r *aper.Reader) (ERABID, error) {
	ext, err := r.ReadBool()
	if err != nil {
		return 0, err
	}

	if ext {
		return 0, fmt.Errorf("s1ap: unsupported E-RAB-ID extension value")
	}

	v, err := r.ReadConstrainedInt(0, 15)

	return ERABID(v), err
}

// QCI ::= INTEGER (0..255).
type QCI uint8

func (q QCI) encode(w *aper.Writer) error {
	return w.WriteConstrainedInt(int64(q), 0, 255)
}

func decodeQCI(r *aper.Reader) (QCI, error) {
	v, err := r.ReadConstrainedInt(0, 255)
	return QCI(v), err
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

// preemptionRootCount is the number of root values of both pre-emption
// ENUMERATEDs (TS 36.413 §9.2.1.60/§9.2.1.61).
const preemptionRootCount = 2

// AllocationAndRetentionPriority ::= SEQUENCE { priorityLevel,
// pre-emptionCapability, pre-emptionVulnerability, iE-Extensions OPTIONAL }
// (extensible). PriorityLevel ::= INTEGER (0..15).
type AllocationAndRetentionPriority struct {
	PriorityLevel           uint8
	PreemptionCapability    PreemptionCapability
	PreemptionVulnerability PreemptionVulnerability
}

func (a AllocationAndRetentionPriority) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteConstrainedInt(int64(a.PriorityLevel), 0, 15); err != nil {
		return err
	}

	if err := w.WriteEnum(int(a.PreemptionCapability), preemptionRootCount, false, false); err != nil {
		return err
	}

	return w.WriteEnum(int(a.PreemptionVulnerability), preemptionRootCount, false, false)
}

func decodeARP(r *aper.Reader) (AllocationAndRetentionPriority, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return AllocationAndRetentionPriority{}, err
	}

	pl, err := r.ReadConstrainedInt(0, 15)
	if err != nil {
		return AllocationAndRetentionPriority{}, err
	}

	preCap, _, err := r.ReadEnum(preemptionRootCount, false)
	if err != nil {
		return AllocationAndRetentionPriority{}, err
	}

	vuln, _, err := r.ReadEnum(preemptionRootCount, false)
	if err != nil {
		return AllocationAndRetentionPriority{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return AllocationAndRetentionPriority{}, err
	}

	return AllocationAndRetentionPriority{
		PriorityLevel:           uint8(pl),
		PreemptionCapability:    PreemptionCapability(preCap),
		PreemptionVulnerability: PreemptionVulnerability(vuln),
	}, nil
}

// GBRQosInformation ::= SEQUENCE { four BitRates, iE-Extensions OPTIONAL }
// (extensible).
type GBRQosInformation struct {
	MaximumBitrateDL    BitRate
	MaximumBitrateUL    BitRate
	GuaranteedBitrateDL BitRate
	GuaranteedBitrateUL BitRate
}

func (g GBRQosInformation) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	for _, b := range []BitRate{g.MaximumBitrateDL, g.MaximumBitrateUL, g.GuaranteedBitrateDL, g.GuaranteedBitrateUL} {
		if err := b.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeGBRQosInformation(r *aper.Reader) (GBRQosInformation, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return GBRQosInformation{}, err
	}

	var rates [4]BitRate

	for i := range rates {
		rates[i], err = decodeBitRate(r)
		if err != nil {
			return GBRQosInformation{}, err
		}
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return GBRQosInformation{}, err
	}

	return GBRQosInformation{
		MaximumBitrateDL: rates[0], MaximumBitrateUL: rates[1],
		GuaranteedBitrateDL: rates[2], GuaranteedBitrateUL: rates[3],
	}, nil
}

// ERABLevelQoSParameters ::= SEQUENCE { qCI, allocationRetentionPriority,
// gbrQosInformation OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type ERABLevelQoSParameters struct {
	QCI QCI
	ARP AllocationAndRetentionPriority
	GBR *GBRQosInformation
}

func (q ERABLevelQoSParameters) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{q.GBR != nil, false})

	if err := q.QCI.encode(w); err != nil {
		return err
	}

	if err := q.ARP.encode(w); err != nil {
		return err
	}

	if q.GBR != nil {
		if err := q.GBR.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeERABLevelQoSParameters(r *aper.Reader) (ERABLevelQoSParameters, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 2)
	if err != nil {
		return ERABLevelQoSParameters{}, err
	}

	qci, err := decodeQCI(r)
	if err != nil {
		return ERABLevelQoSParameters{}, err
	}

	arp, err := decodeARP(r)
	if err != nil {
		return ERABLevelQoSParameters{}, err
	}

	out := ERABLevelQoSParameters{QCI: qci, ARP: arp}

	if opt[0] {
		gbr, err := decodeGBRQosInformation(r)
		if err != nil {
			return ERABLevelQoSParameters{}, err
		}

		out.GBR = &gbr
	}

	if err := skipSequenceExtensions(r, opt[1], extPresent); err != nil {
		return ERABLevelQoSParameters{}, err
	}

	return out, nil
}

// TransportLayerAddress ::= BIT STRING (SIZE(1..160, ...)). Holds the address
// octets (IPv4 = 4, IPv6 = 16).
type TransportLayerAddress []byte

func (a TransportLayerAddress) encode(w *aper.Writer) error {
	return w.WriteBitString(a, len(a)*8, 1, 160, true)
}

func decodeTransportLayerAddress(r *aper.Reader) (TransportLayerAddress, error) {
	b, nbits, err := r.ReadBitString(1, 160, true)
	if err != nil {
		return nil, err
	}

	return TransportLayerAddress(b[:(nbits+7)/8]), nil
}

// GTPTEID ::= OCTET STRING (SIZE(4)).
type GTPTEID uint32

func (t GTPTEID) encode(w *aper.Writer) error {
	return w.WriteOctetString([]byte{byte(t >> 24), byte(t >> 16), byte(t >> 8), byte(t)}, 4, 4, false)
}

func decodeGTPTEID(r *aper.Reader) (GTPTEID, error) {
	b, err := r.ReadOctetString(4, 4, false)
	if err != nil {
		return 0, err
	}

	return GTPTEID(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])), nil
}

// SecurityKey ::= BIT STRING (SIZE(256)).
type SecurityKey [32]byte

func (k SecurityKey) encode(w *aper.Writer) error {
	return w.WriteBitString(k[:], 256, 256, 256, false)
}

func decodeSecurityKey(r *aper.Reader) (SecurityKey, error) {
	b, _, err := r.ReadBitString(256, 256, false)
	if err != nil {
		return SecurityKey{}, err
	}

	var k SecurityKey

	copy(k[:], b)

	return k, nil
}

// UESecurityCapabilities ::= SEQUENCE { encryptionAlgorithms,
// integrityProtectionAlgorithms, iE-Extensions OPTIONAL } (extensible). Each
// algorithm field is BIT STRING (SIZE(16, ...)).
type UESecurityCapabilities struct {
	EncryptionAlgorithms          uint16
	IntegrityProtectionAlgorithms uint16
}

func (c UESecurityCapabilities) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteBitString(uintToBits(uint64(c.EncryptionAlgorithms), 16), 16, 16, 16, true); err != nil {
		return err
	}

	return w.WriteBitString(uintToBits(uint64(c.IntegrityProtectionAlgorithms), 16), 16, 16, 16, true)
}

func decodeUESecurityCapabilities(r *aper.Reader) (UESecurityCapabilities, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return UESecurityCapabilities{}, err
	}

	enc, encBits, err := r.ReadBitString(16, 16, true)
	if err != nil {
		return UESecurityCapabilities{}, err
	}

	integ, integBits, err := r.ReadBitString(16, 16, true)
	if err != nil {
		return UESecurityCapabilities{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return UESecurityCapabilities{}, err
	}

	return UESecurityCapabilities{
		EncryptionAlgorithms:          uint16(bitsToUint(enc, encBits)),
		IntegrityProtectionAlgorithms: uint16(bitsToUint(integ, integBits)),
	}, nil
}

// ERABToBeSetupItemCtxtSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU OPTIONAL, iE-Extensions OPTIONAL }
// (extensible). A nil NASPDU means the optional field is absent.
type ERABToBeSetupItemCtxtSUReq struct {
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU
}

func (it ERABToBeSetupItemCtxtSUReq) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{it.NASPDU != nil, false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.QoS.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	if err := it.GTPTEID.encode(w); err != nil {
		return err
	}

	if it.NASPDU != nil {
		return it.NASPDU.encode(w)
	}

	return nil
}

func decodeERABToBeSetupItemCtxtSUReq(r *aper.Reader) (ERABToBeSetupItemCtxtSUReq, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 2)
	if err != nil {
		return ERABToBeSetupItemCtxtSUReq{}, err
	}

	var it ERABToBeSetupItemCtxtSUReq

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.QoS, err = decodeERABLevelQoSParameters(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.GTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if opt[0] {
		if it.NASPDU, err = decodeNASPDU(r); err != nil {
			return it, err
		}
	}

	if err := skipSequenceExtensions(r, opt[1], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ERABSetupItemCtxtSURes ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible).
type ERABSetupItemCtxtSURes struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
}

func (it ERABSetupItemCtxtSURes) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	return it.GTPTEID.encode(w)
}

func decodeERABSetupItemCtxtSURes(r *aper.Reader) (ERABSetupItemCtxtSURes, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABSetupItemCtxtSURes{}, err
	}

	var it ERABSetupItemCtxtSURes

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.GTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ERABItem ::= SEQUENCE { e-RAB-ID, cause, iE-Extensions OPTIONAL } (extensible).
// Used in failed-to-setup lists.
type ERABItem struct {
	ERABID ERABID
	Cause  Cause
}

func (it ERABItem) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	return it.Cause.encode(w)
}

func decodeERABItem(r *aper.Reader) (ERABItem, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABItem{}, err
	}

	var it ERABItem

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.Cause, err = decodeCause(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}
