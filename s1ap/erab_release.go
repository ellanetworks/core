// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// ERABReleaseItemBearerRelComp ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible): an E-RAB the eNB confirms released (TS 36.413).
type ERABReleaseItemBearerRelComp struct {
	ERABID ERABID
}

func (it ERABReleaseItemBearerRelComp) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	return it.ERABID.encode(w)
}

func decodeERABReleaseItemBearerRelComp(r *aper.Reader) (ERABReleaseItemBearerRelComp, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABReleaseItemBearerRelComp{}, err
	}

	var it ERABReleaseItemBearerRelComp

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ERABReleaseCommand is the E-RAB RELEASE COMMAND message (TS 36.413),
// sent by the MME to release one or more E-RABs of a UE that stays connected —
// the radio leg of a PDN connection being disconnected (TS 23.401,
// "Deactivate Bearer Request"). The DEACTIVATE EPS BEARER CONTEXT REQUEST NAS
// message rides in the optional NAS-PDU IE, so the eNB releases the radio bearer
// and delivers the NAS in one step.
type ERABReleaseCommand struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeReleased          []ERABItem
	NASPDU                    NASPDU

	unmodeledIEs
}

func (m *ERABReleaseCommand) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, enc: ambr.encode})
	}

	fields = append(fields, ieField{id: idERABToBeReleasedList, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
		return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABToBeReleased))
	}})

	if len(m.NASPDU) > 0 {
		fields = append(fields, ieField{id: idNASPDU, crit: CriticalityIgnore, enc: m.NASPDU.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABReleaseCommand) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABReleaseCommand decodes the message from an initiatingMessage open-type
// payload.
func ParseERABReleaseCommand(value []byte) (*ERABReleaseCommand, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABReleaseCommand preamble: %w", err)
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

	m := &ERABReleaseCommand{}

	var seenMME, seenENB, seenList bool

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
		case idERABToBeReleasedList:
			m.ERABToBeReleased, err = decodeERABItemList(sub)
			seenList = true
		case idNASPDU:
			m.NASPDU, err = decodeNASPDU(sub)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABReleaseCommand IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenList {
		return nil, fmt.Errorf("s1ap: ERABReleaseCommand missing mandatory IE")
	}

	return m, nil
}

// ERABReleaseResponse is the E-RAB RELEASE RESPONSE message (TS 36.413),
// sent by the eNB once the E-RAB(s) are released.
type ERABReleaseResponse struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	ERABReleased           []ERABReleaseItemBearerRelComp
	ERABFailedToRelease    []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ERABReleaseResponse) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
	}

	if len(m.ERABReleased) > 0 {
		fields = append(fields, ieField{id: idERABReleaseListBearerRelComp, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABReleaseItemBearerRelComp, CriticalityIgnore, encoderList(m.ERABReleased))
		}})
	}

	if len(m.ERABFailedToRelease) > 0 {
		fields = append(fields, ieField{id: idERABFailedToReleaseList, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABFailedToRelease))
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
func (m *ERABReleaseResponse) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABReleaseResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABReleaseResponse(value []byte) (*ERABReleaseResponse, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABReleaseResponse preamble: %w", err)
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

	m := &ERABReleaseResponse{}

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
		case idERABReleaseListBearerRelComp:
			m.ERABReleased, err = decodeERABReleaseList(sub)
		case idERABFailedToReleaseList:
			m.ERABFailedToRelease, err = decodeERABItemList(sub)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABReleaseResponse IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB {
		return nil, fmt.Errorf("s1ap: ERABReleaseResponse missing mandatory IE")
	}

	return m, nil
}

func decodeERABReleaseList(r *aper.Reader) ([]ERABReleaseItemBearerRelComp, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABReleaseItemBearerRelComp)
}
