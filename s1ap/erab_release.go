// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ERABReleaseItemBearerRelComp ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible): an E-RAB the eNB confirms released (TS 36.413).
type ERABReleaseItemBearerRelComp struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	_      ieExtensions `per:",skip"`
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

func (m *ERABReleaseCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, val: &ambr})
	}

	fields = append(fields, ieField{id: idERABToBeReleasedList, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
		return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABToBeReleased)
	})})

	if len(m.NASPDU) > 0 {
		fields = append(fields, ieField{id: idNASPDU, crit: CriticalityIgnore, val: &m.NASPDU})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABReleaseCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABReleaseCommand decodes the message from an initiatingMessage open-type
// payload.
func ParseERABReleaseCommand(value []byte) (*ERABReleaseCommand, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABReleaseCommand preamble: %w", err)
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

	m := &ERABReleaseCommand{}

	var seenMME, seenENB, seenList bool

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
		case idERABToBeReleasedList:
			m.ERABToBeReleased, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
			seenList = true
		case idNASPDU:
			err = perIEDecode(f.value, &m.NASPDU)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABReleaseCommand IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcERABRelease,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idERABToBeReleasedList, CriticalityIgnore, seenList},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// ERABReleaseResponse is the E-RAB RELEASE RESPONSE message (TS 36.413),
// sent by the eNB once the E-RAB(s) are released.
type ERABReleaseResponse struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ERABReleased            []ERABReleaseItemBearerRelComp
	ERABFailedToRelease     []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	unmodeledIEs
}

func (m *ERABReleaseResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
	}

	if len(m.ERABReleased) > 0 {
		fields = append(fields, ieField{id: idERABReleaseListBearerRelComp, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABReleaseItemBearerRelComp, CriticalityIgnore, m.ERABReleased)
		})})
	}

	if len(m.ERABFailedToRelease) > 0 {
		fields = append(fields, ieField{id: idERABFailedToReleaseList, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToRelease)
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
func (m *ERABReleaseResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABReleaseResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABReleaseResponse(value []byte) (*ERABReleaseResponse, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABReleaseResponse preamble: %w", err)
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

	m := &ERABReleaseResponse{}

	var seenMME, seenENB bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABReleaseListBearerRelComp:
			m.ERABReleased, err = decodeItemList[ERABReleaseItemBearerRelComp](per.NewReader(f.value), enc, maxnoofERABs)
		case idERABFailedToReleaseList:
			m.ERABFailedToRelease, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
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
			return nil, fmt.Errorf("s1ap: ERABReleaseResponse IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcERABRelease,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
	); err != nil {
		return nil, err
	}

	return m, nil
}
