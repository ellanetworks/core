// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ERABToBeModifiedItemBearerModReq ::= SEQUENCE { e-RAB-ID,
// e-RABLevelQoSParameters, nAS-PDU, iE-Extensions OPTIONAL } (extensible). The
// NAS-PDU carries the MODIFY EPS BEARER CONTEXT REQUEST for the bearer
// (TS 36.413). Unlike E-RAB Setup there is no transport layer address:
// the S1-U endpoint is unchanged.
type ERABToBeModifiedItemBearerModReq struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	QoS    ERABLevelQoSParameters
	NASPDU NASPDU
	_      ieExtensions `per:",skip"`
}

// ERABModifyItemBearerModRes ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible): one successfully modified E-RAB in the E-RAB MODIFY RESPONSE
// (TS 36.413).
type ERABModifyItemBearerModRes struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	_      ieExtensions `per:",skip"`
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

func (m *ERABModifyRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, val: &ambr})
	}

	fields = append(fields, ieField{id: idERABToBeModifiedListBearerModReq, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
		return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeModifiedItemBearerModReq, CriticalityReject, m.ERABToBeModified)
	})})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABModifyRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABModifyRequest decodes the message from an initiatingMessage open-type
// payload.
func ParseERABModifyRequest(value []byte) (*ERABModifyRequest, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABModifyRequest preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &ERABModifyRequest{}

	var seenMME, seenENB, seenERAB bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			var ambr UEAggregateMaximumBitRate

			err = perIEDecode(f.value, &ambr)
			m.UEAggregateMaximumBitRate = &ambr
		case idERABToBeModifiedListBearerModReq:
			m.ERABToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModReq](per.NewReader(f.value), enc, maxnoofERABs)
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

// ERABModifyResponse is the E-RAB MODIFY RESPONSE message (TS 36.413),
// sent by the eNB once the radio bearer QoS is reconfigured. ERABModify lists the
// successfully modified E-RABs; ERABFailedToModify lists those rejected.
type ERABModifyResponse struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ERABModify              []ERABModifyItemBearerModRes
	ERABFailedToModify      []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	unmodeledIEs
}

func (m *ERABModifyResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
	}

	if len(m.ERABModify) > 0 {
		fields = append(fields, ieField{id: idERABModifyListBearerModRes, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABModifyItemBearerModRes, CriticalityIgnore, m.ERABModify)
		})})
	}

	if len(m.ERABFailedToModify) > 0 {
		fields = append(fields, ieField{id: idERABFailedToModifyList, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToModify)
		})})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: &d})
	}

	if m.UserLocationInformation != nil {
		u := *m.UserLocationInformation
		fields = append(fields, ieField{id: idUserLocationInformation, crit: CriticalityIgnore, val: &u})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABModifyResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABModifyResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABModifyResponse(value []byte) (*ERABModifyResponse, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABModifyResponse preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &ERABModifyResponse{}

	var seenMME, seenENB bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABModifyListBearerModRes:
			m.ERABModify, err = decodeItemList[ERABModifyItemBearerModRes](per.NewReader(f.value), enc, maxnoofERABs)
		case idERABFailedToModifyList:
			m.ERABFailedToModify, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		case idUserLocationInformation:
			var uli UserLocationInformation

			err = perIEDecode(f.value, &uli)
			m.UserLocationInformation = &uli
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
