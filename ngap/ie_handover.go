// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// HandoverType ::= ENUMERATED { intra5gs, fivegs-to-eps, eps-to-5gs, ...,
// fivegs-to-utran } — TS 38.413 §9.3.1.22. TS 36.413 §9.2.1.13 names five
// entirely different members (intralte, ltetoutran, ltetogeran, utrantolte,
// gerantolte), so the two enumerations share no wire values.
type HandoverType uint8

const (
	HandoverTypeIntra5GS HandoverType = iota
	HandoverTypeFiveGSToEPS
	HandoverTypeEPSToFiveGS

	handoverTypeRootCount = 3
)

func (h HandoverType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, handoverTypeRootCount, int64(h), "HandoverType")
}

func (h *HandoverType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, handoverTypeRootCount, "HandoverType")
	if err != nil {
		return err
	}

	*h = HandoverType(idx)

	return nil
}

// SourceToTargetTransparentContainer ::= OCTET STRING — TS 38.413 §9.3.1.20.
// Opaque here: the octets are encoded per the target system's own spec.
type SourceToTargetTransparentContainer []byte

func (c SourceToTargetTransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *SourceToTargetTransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = SourceToTargetTransparentContainer(b)

	return nil
}

// TargetToSourceTransparentContainer ::= OCTET STRING — TS 38.413 §9.3.1.21.
// Opaque here: the octets are encoded per the target system's own spec.
type TargetToSourceTransparentContainer []byte

func (c TargetToSourceTransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TargetToSourceTransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TargetToSourceTransparentContainer(b)

	return nil
}

// NASSecurityParametersFromNGRAN ::= OCTET STRING — TS 38.413 §9.3.3.26. The
// octets are the NAS security parameters TS 33.501 §8.4 defines for a handover
// out of 5GS; NGAP relays them without looking inside.
type NASSecurityParametersFromNGRAN []byte

func (p NASSecurityParametersFromNGRAN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NASSecurityParametersFromNGRAN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NASSecurityParametersFromNGRAN(b)

	return nil
}

// TargetRANNodeID ::= SEQUENCE { globalRANNodeID, selectedTAI, iE-Extensions
// OPTIONAL } (extensible), the NG-RAN alternative of the Target ID IE
// (TS 38.413 §9.3.1.25).
type TargetRANNodeID struct {
	_               [0]struct{} `per:"extseq"`
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     TAI
	_               ieExtensions `per:",skip"`
}

// TargetID CHOICE alternatives. The CHOICE is not extensible, so
// choice-Extensions is a plain alternative index.
const (
	targetIDTargetRANNodeID = iota
	targetIDTargetENBID
	targetIDChoiceExtensions

	targetIDAlternatives = 3
)

// TargetID ::= CHOICE { targetRANNodeID, targeteNB-ID, choice-Extensions } —
// TS 38.413 §9.3.1.25. Only targetRANNodeID is modelled: Ella hands over within
// 5GS, so a targeteNB-ID names a target it cannot reach. It is refused rather
// than misread. TS 36.413's TargetID is a different CHOICE entirely
// (targeteNB-ID, targetRNC-ID, cGI).
type TargetID struct {
	TargetRANNodeID TargetRANNodeID
}

func (t TargetID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, targetIDAlternatives-1, targetIDTargetRANNodeID); err != nil {
		return err
	}

	return t.TargetRANNodeID.MarshalPER(w, enc)
}

func (t *TargetID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, targetIDAlternatives-1)
	if err != nil {
		return err
	}

	switch idx {
	case targetIDTargetRANNodeID:
		return t.TargetRANNodeID.UnmarshalPER(r, enc)
	case targetIDChoiceExtensions:
		return decodeChoiceExtension(r, enc, "TargetID")
	default:
		return fmt.Errorf("%w: TargetID targeteNB-ID", errNotComprehended)
	}
}

// PDUSessionResourceItemHORqd ::= SEQUENCE { pDUSessionID,
// handoverRequiredTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceItemHORqd struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceListHORqd ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceItemHORqd.
type PDUSessionResourceListHORqd []PDUSessionResourceItemHORqd

// PDUSessionResourceHandoverItem ::= SEQUENCE { pDUSessionID,
// handoverCommandTransfer, iE-Extensions OPTIONAL } (extensible). One entry per
// session the target admitted.
type PDUSessionResourceHandoverItem struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceHandoverList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceHandoverItem.
type PDUSessionResourceHandoverList []PDUSessionResourceHandoverItem

// PDUSessionResourceToReleaseItemHOCmd ::= SEQUENCE { pDUSessionID,
// handoverPreparationUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible). One entry per session the target refused.
type PDUSessionResourceToReleaseItemHOCmd struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceToReleaseListHOCmd ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceToReleaseItemHOCmd.
type PDUSessionResourceToReleaseListHOCmd []PDUSessionResourceToReleaseItemHOCmd

// TargettoSourceFailureTransparentContainer ::= OCTET STRING — TS 38.413
// §9.3.1.186. The target returns it when it cannot admit the handover, and the
// AMF relays it to the source. TS 36.413 has no counterpart.
type TargettoSourceFailureTransparentContainer []byte

func (c TargettoSourceFailureTransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TargettoSourceFailureTransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TargettoSourceFailureTransparentContainer(b)

	return nil
}

// SecurityContext ::= SEQUENCE { nextHopChainingCount, nextHopNH,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.88. The {NH, NCC}
// pair the target derives its AS keys from (TS 33.501).
type SecurityContext struct {
	_                    [0]struct{} `per:"extseq"`
	NextHopChainingCount uint8       `per:",range:0..7"`
	NextHopNH            SecurityKey
	_                    ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupItemHOReq ::= SEQUENCE { pDUSessionID, s-NSSAI,
// handoverRequestTransfer, iE-Extensions OPTIONAL } (extensible). The transfer
// is a PDUSessionResourceSetupRequestTransfer, the same one the setup procedure
// carries.
type PDUSessionResourceSetupItemHOReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	SNSSAI       SNSSAI
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListHOReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemHOReq.
type PDUSessionResourceSetupListHOReq []PDUSessionResourceSetupItemHOReq

// PDUSessionResourceAdmittedItem ::= SEQUENCE { pDUSessionID,
// handoverRequestAcknowledgeTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceAdmittedItem struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceAdmittedList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceAdmittedItem.
type PDUSessionResourceAdmittedList []PDUSessionResourceAdmittedItem

// PDUSessionResourceFailedToSetupItemHOAck ::= SEQUENCE { pDUSessionID,
// handoverResourceAllocationUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToSetupItemHOAck struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToSetupListHOAck ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemHOAck.
type PDUSessionResourceFailedToSetupListHOAck []PDUSessionResourceFailedToSetupItemHOAck

// PDUSessionResourceToBeSwitchedDLItem ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceToBeSwitchedDLItem struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceToBeSwitchedDLList ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceToBeSwitchedDLItem.
type PDUSessionResourceToBeSwitchedDLList []PDUSessionResourceToBeSwitchedDLItem

// PDUSessionResourceFailedToSetupItemPSReq ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestSetupFailedTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceFailedToSetupItemPSReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToSetupListPSReq ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemPSReq.
type PDUSessionResourceFailedToSetupListPSReq []PDUSessionResourceFailedToSetupItemPSReq

// PDUSessionResourceSwitchedItem ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestAcknowledgeTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceSwitchedItem struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSwitchedList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceSwitchedItem.
type PDUSessionResourceSwitchedList []PDUSessionResourceSwitchedItem

// PDUSessionResourceReleasedItemPSAck ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestUnsuccessfulTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceReleasedItemPSAck struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceReleasedListPSAck ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceReleasedItemPSAck.
type PDUSessionResourceReleasedListPSAck []PDUSessionResourceReleasedItemPSAck

// PDUSessionResourceReleasedItemPSFail ::= SEQUENCE { pDUSessionID,
// pathSwitchRequestUnsuccessfulTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceReleasedItemPSFail struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceReleasedListPSFail ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceReleasedItemPSFail.
type PDUSessionResourceReleasedListPSFail []PDUSessionResourceReleasedItemPSFail
