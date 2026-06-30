// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// ERABToBeModifiedItemBearerModReq ::= SEQUENCE { e-RAB-ID,
// e-RABLevelQoSParameters, nAS-PDU, iE-Extensions OPTIONAL } (extensible). The
// NAS-PDU carries the MODIFY EPS BEARER CONTEXT REQUEST for the bearer
// (TS 36.413). Unlike E-RAB Setup there is no transport layer address:
// the S1-U endpoint is unchanged.
type ERABToBeModifiedItemBearerModReq struct {
	ERABID ERABID
	QoS    ERABLevelQoSParameters
	NASPDU NASPDU
}

func (it ERABToBeModifiedItemBearerModReq) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.QoS.encode(w); err != nil {
		return err
	}

	return it.NASPDU.encode(w)
}

func decodeERABToBeModifiedItemBearerModReq(r *aper.Reader) (ERABToBeModifiedItemBearerModReq, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABToBeModifiedItemBearerModReq{}, err
	}

	var it ERABToBeModifiedItemBearerModReq

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.QoS, err = decodeERABLevelQoSParameters(r); err != nil {
		return it, err
	}

	if it.NASPDU, err = decodeNASPDU(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ERABModifyItemBearerModRes ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible): one successfully modified E-RAB in the E-RAB MODIFY RESPONSE
// (TS 36.413).
type ERABModifyItemBearerModRes struct {
	ERABID ERABID
}

func (it ERABModifyItemBearerModRes) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	return it.ERABID.encode(w)
}

func decodeERABModifyItemBearerModRes(r *aper.Reader) (ERABModifyItemBearerModRes, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABModifyItemBearerModRes{}, err
	}

	var it ERABModifyItemBearerModRes

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ERABModifyRequest is the E-RAB MODIFY REQUEST message (TS 36.413),
// sent by the MME to change the QoS of one or more active E-RABs. The new
// E-RAB-level QoS (QCI, ARP) reconfigures the radio bearer; the piggybacked
// NAS-PDU carries the MODIFY EPS BEARER CONTEXT REQUEST to the UE.
type ERABModifyRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeModified          []ERABToBeModifiedItemBearerModReq

	unmodeledIEs
}

func (m *ERABModifyRequest) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, enc: ambr.encode})
	}

	fields = append(fields, ieField{id: idERABToBeModifiedListBearerModReq, crit: CriticalityReject, enc: func(w *aper.Writer) error {
		return encodeSingleContainerList(w, maxnoofERABs, idERABToBeModifiedItemBearerModReq, CriticalityReject, encoderList(m.ERABToBeModified))
	}})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABModifyRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABModifyRequest decodes the message from an initiatingMessage open-type
// payload.
func ParseERABModifyRequest(value []byte) (*ERABModifyRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABModifyRequest preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &ERABModifyRequest{}

	var seenMME, seenENB, seenERAB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			var ambr UEAggregateMaximumBitRate

			ambr, err = decodeUEAggregateMaximumBitRate(sub)
			m.UEAggregateMaximumBitRate = &ambr
		case idERABToBeModifiedListBearerModReq:
			m.ERABToBeModified, err = decodeERABToBeModifiedList(sub)
			seenERAB = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABModifyRequest IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenERAB {
		return nil, fmt.Errorf("s1ap: ERABModifyRequest missing mandatory IE")
	}

	return m, nil
}

func decodeERABToBeModifiedList(r *aper.Reader) ([]ERABToBeModifiedItemBearerModReq, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABToBeModifiedItemBearerModReq)
}

// ERABModifyResponse is the E-RAB MODIFY RESPONSE message (TS 36.413),
// sent by the eNB once the radio bearer QoS is reconfigured. ERABModify lists the
// successfully modified E-RABs; ERABFailedToModify lists those rejected.
type ERABModifyResponse struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	ERABModify             []ERABModifyItemBearerModRes
	ERABFailedToModify     []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ERABModifyResponse) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
	}

	if len(m.ERABModify) > 0 {
		fields = append(fields, ieField{id: idERABModifyListBearerModRes, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABModifyItemBearerModRes, CriticalityIgnore, encoderList(m.ERABModify))
		}})
	}

	if len(m.ERABFailedToModify) > 0 {
		fields = append(fields, ieField{id: idERABFailedToModifyList, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABFailedToModify))
		}})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, enc: d.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABModifyResponse) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABModifyResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABModifyResponse(value []byte) (*ERABModifyResponse, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABModifyResponse preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &ERABModifyResponse{}

	var seenMME, seenENB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idERABModifyListBearerModRes:
			m.ERABModify, err = decodeERABModifyList(sub)
		case idERABFailedToModifyList:
			m.ERABFailedToModify, err = decodeERABItemList(sub)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABModifyResponse IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB {
		return nil, fmt.Errorf("s1ap: ERABModifyResponse missing mandatory IE")
	}

	return m, nil
}

func decodeERABModifyList(r *aper.Reader) ([]ERABModifyItemBearerModRes, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABModifyItemBearerModRes)
}
