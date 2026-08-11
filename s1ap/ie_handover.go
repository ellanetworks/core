// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// HandoverType ::= ENUMERATED { intralte, ltetoutran, ltetogeran, utrantolte,
// gerantolte, ..., eps-to-5gs, fivegs-to-eps } (extensible) (TS 36.413
// §9.2.1.13).
//
// The two inter-system values are *extension additions* here, where TS 38.413
// §9.3.1.22 has its own as root values — the enumerations are not symmetric, and
// neither their members nor their wire encodings correspond. An extension
// addition is encoded as the extension bit followed by a normally-small index
// into the additions, so eps-to-5gs and fivegs-to-eps do not share a wire form
// with any root value.
type HandoverType uint8

const (
	HandoverTypeIntraLTE HandoverType = iota
	HandoverTypeLTEtoUTRAN
	HandoverTypeLTEtoGERAN
	HandoverTypeUTRANtoLTE
	HandoverTypeGERANtoLTE

	// The extension additions, numbered on from the root as PER numbers them:
	// the k-th addition is handoverTypeRootCount+k.
	HandoverTypeEPSToFiveGS
	HandoverTypeFiveGSToEPS
)

const (
	handoverTypeRootCount = 5
	// handoverTypeCount is every value 3GPP has assigned, root and extension
	// together. A value beyond it names no handover this release defines.
	handoverTypeCount = 7
)

func (t HandoverType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if t >= handoverTypeCount {
		return fmt.Errorf("s1ap: HandoverType %d outside the assigned values", t)
	}

	return per.EncodeEnumerated(w, enc, handoverTypeRootCount, true, int64(t))
}

func (t *HandoverType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeEnumerated(r, enc, handoverTypeRootCount, true)
	if err != nil {
		return err
	}

	// §10.3.1 case 6, handled on the IE's criticality: an addition a later
	// release makes is refused rather than narrowed onto a root value.
	if idx >= handoverTypeCount {
		return fmt.Errorf("%w: HandoverType extension value %d", errNotComprehended, idx)
	}

	*t = HandoverType(idx)

	return nil
}

// TransparentContainer ::= OCTET STRING, relayed opaquely; the bytes are RAN
// RRC information (TS 36.300).
type TransparentContainer []byte

func (c TransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TransparentContainer(b)

	return nil
}

// NASSecurityParametersfromE-UTRAN ::= OCTET STRING (TS 36.413 §9.2.3.30). The
// octets are the NAS security parameters TS 33.401 §9.2 defines for a handover
// out of E-UTRAN; S1AP relays them without looking inside.
type NASSecurityParametersfromEUTRAN []byte

func (p NASSecurityParametersfromEUTRAN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NASSecurityParametersfromEUTRAN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NASSecurityParametersfromEUTRAN(b)

	return nil
}

// NASSecurityParameterstoE-UTRAN ::= OCTET STRING (TS 36.413 §9.2.3.31). The
// mirror of the from-E-UTRAN parameters, sent on a handover into E-UTRAN.
type NASSecurityParameterstoEUTRAN []byte

func (p NASSecurityParameterstoEUTRAN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NASSecurityParameterstoEUTRAN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NASSecurityParameterstoEUTRAN(b)

	return nil
}

// SecurityContext ::= SEQUENCE { nextHopChainingCount, nextHopParameter,
// iE-Extensions OPTIONAL } (extensible) (TS 36.413 §9.2.1.26). The {NH, NCC}
// pair the target derives the next KeNB from (TS 33.401).
type SecurityContext struct {
	_                    [0]struct{} `per:"extseq"`
	NextHopChainingCount uint8       `per:",range:0..7"`
	NextHopParameter     SecurityKey
	_                    ieExtensions `per:",skip"`
}

// TargeteNB-ID ::= SEQUENCE { global-ENB-ID, selected-TAI, iE-Extensions
// OPTIONAL } (extensible) (TS 36.413).
type TargeteNBID struct {
	_           [0]struct{} `per:"extseq"`
	GlobalENBID GlobalENBID
	SelectedTAI TAI
	_           ieExtensions `per:",skip"`
}

// TargetNgRanNode-ID ::= SEQUENCE { global-RAN-NODE-ID, selected-TAI FiveGSTAI,
// iE-Extensions OPTIONAL } (extensible) (TS 36.413). It names an NG-RAN node an
// EPS to 5GS handover targets, so the tracking area it carries is a 5GS one.
type TargetNgRanNodeID struct {
	_               [0]struct{} `per:"extseq"`
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     FiveGSTAI
	_               ieExtensions `per:",skip"`
}

// TargetID ::= CHOICE { targeteNB-ID, targetRNC-ID, cGI, ...,
// targetgNgRanNode-ID } (TS 36.413). Two alternatives are modelled: targeteNB-ID
// for an intra-LTE handover and the targetgNgRanNode-ID extension addition for
// the EPS to 5GS one. targetRNC-ID and cGI name targets Ella cannot reach.
//
// Which alternative is present is not free: TS 36.413 §9.1.5.1 pairs it with the
// Handover Type IE, so an eps-to-5gs Handover Required carries the NG-RAN target
// and an intralte one the eNB target. The CHOICE does not enforce that — the
// message handler does.
type TargetID struct {
	TargeteNBID       TargeteNBID
	TargetNgRanNodeID *TargetNgRanNodeID
}

const (
	targetIDRootCount = 3 // targeteNB-ID, targetRNC-ID, cGI

	// targetIDExtTargetNgRanNodeID is the extension addition's index: the first,
	// and so far only, addition TS 36.413 makes to the CHOICE.
	targetIDExtTargetNgRanNodeID = 0
)

func (t TargetID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if t.TargetNgRanNodeID != nil {
		w.WriteBit(true)

		if err := per.EncodeNormallySmall(w, enc, targetIDExtTargetNgRanNodeID); err != nil {
			return err
		}

		// An extension alternative travels as an open type, so a receiver that
		// does not know it can step over it (X.691 §23).
		inner := per.NewWriter()
		if err := t.TargetNgRanNodeID.MarshalPER(inner, enc); err != nil {
			return err
		}

		inner.AlignToByte()

		return per.EncodeOpenTypeBytes(w, enc, inner.Bytes())
	}

	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, targetIDRootCount-1, 0); err != nil {
		return err
	}

	return t.TargeteNBID.MarshalPER(w, enc)
}

func (t *TargetID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	if isExt {
		return t.unmarshalExtension(r, enc)
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, targetIDRootCount-1)
	if err != nil {
		return err
	}

	if idx != 0 {
		return fmt.Errorf("%w: TargetID alternative %d (only targeteNB-ID)", errNotComprehended, idx)
	}

	return t.TargeteNBID.UnmarshalPER(r, enc)
}

func (t *TargetID) unmarshalExtension(r *per.Reader, enc per.Encoding) error {
	extIdx, err := per.DecodeNormallySmall(r, enc)
	if err != nil {
		return err
	}

	// The open type is read whatever it turns out to hold, so an addition a later
	// release makes leaves the reader positioned after the alternative it could
	// not decode.
	raw, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return err
	}

	if extIdx != targetIDExtTargetNgRanNodeID {
		return fmt.Errorf("%w: TargetID extension alternative %d", errNotComprehended, extIdx)
	}

	var node TargetNgRanNodeID
	if err := node.UnmarshalPER(per.NewReader(raw), enc); err != nil {
		return err
	}

	t.TargetNgRanNodeID = &node

	return nil
}

// ERABToBeSetupItemHOReq ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, e-RABlevelQosParameters, iE-Extensions OPTIONAL } (extensible)
// (TS 36.413).
type ERABToBeSetupItemHOReq struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	QoS                   ERABLevelQoSParameters
	Extensions            *ERABToBeSetupItemHOReqExtIEs `per:",optional"`
}

// DataForwardingNotPossible ::= ENUMERATED { data-forwarding-not-possible,
// ... } — TS 36.413 §9.2.1.76.
type DataForwardingNotPossible uint8

const (
	DataForwardingNotPossibleTrue DataForwardingNotPossible = iota

	dataForwardingNotPossibleRootCount = 1
)

// ERABToBeSetupItemHOReqExtIEs is the ProtocolExtensionContainer of
// ERABToBeSetupItemHOReq. Of the extensions TS 36.413 defines for it, only
// id-Data-Forwarding-Not-Possible (143, criticality ignore) is modeled.
type ERABToBeSetupItemHOReqExtIEs struct {
	DataForwardingNotPossible *DataForwardingNotPossible
}

// ERABAdmittedItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress, gTP-TEID,
// dL-transportLayerAddress OPTIONAL, dL-gTP-TEID OPTIONAL, uL-TransportLayerAddress
// OPTIONAL, uL-GTP-TEID OPTIONAL, iE-Extensions OPTIONAL } (extensible)
// (TS 36.413).
type ERABAdmittedItem struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	DLTransportLayerAddr  TransportLayerAddress `per:",optional"`
	DLGTPTEID             *GTPTEID              `per:",optional"`
	ULTransportLayerAddr  TransportLayerAddress `per:",optional"`
	ULGTPTEID             *GTPTEID              `per:",optional"`
	_                     ieExtensions          `per:",skip"`
}

// E-RABDataForwardingItem ::= SEQUENCE { e-RAB-ID, dL-transportLayerAddress
// OPTIONAL, dL-gTP-TEID OPTIONAL, uL-TransportLayerAddress OPTIONAL, uL-GTP-TEID
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) (TS 36.413). The 5G
// counterpart is not an IE of the message body: NGAP carries the same tunnels
// inside the per-session HandoverCommandTransfer.
type ERABDataForwardingItem struct {
	_                    [0]struct{} `per:"extseq"`
	ERABID               ERABID
	DLTransportLayerAddr TransportLayerAddress `per:",optional"`
	DLGTPTEID            *GTPTEID              `per:",optional"`
	ULTransportLayerAddr TransportLayerAddress `per:",optional"`
	ULGTPTEID            *GTPTEID              `per:",optional"`
	_                    ieExtensions          `per:",skip"`
}
