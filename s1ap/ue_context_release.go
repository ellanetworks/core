// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// UES1APIDs is the UE-S1AP-IDs CHOICE (TS 36.413): either the
// UE-S1AP-ID-pair (both identities, the form an MME sends) or a bare
// MME-UE-S1AP-ID. Pair selects which alternative.
type UES1APIDs struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Pair        bool
}

func (u UES1APIDs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	if !u.Pair {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ues1apIDsChoiceRootCount-1, 1); err != nil {
			return err
		}

		return u.MMEUES1APID.MarshalPER(w, enc)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ues1apIDsChoiceRootCount-1, 0); err != nil {
		return err
	}

	// UE-S1AP-ID-pair ::= SEQUENCE { mME-UE-S1AP-ID, eNB-UE-S1AP-ID,
	// iE-Extensions OPTIONAL } (extensible).
	w.WriteBit(false)
	w.WriteBit(false)

	if err := u.MMEUES1APID.MarshalPER(w, enc); err != nil {
		return err
	}

	return u.ENBUES1APID.MarshalPER(w, enc)
}

func (u *UES1APIDs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	if isExt {
		return fmt.Errorf("s1ap: UE-S1AP-IDs extension alternative unsupported")
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, ues1apIDsChoiceRootCount-1)
	if err != nil {
		return err
	}

	switch idx {
	case 0:
		extBit, err := r.ReadBit()
		if err != nil {
			return err
		}

		extContainer, err := r.ReadBit()
		if err != nil {
			return err
		}

		var mme MMEUES1APID
		if err := mme.UnmarshalPER(r, enc); err != nil {
			return err
		}

		var enb ENBUES1APID
		if err := enb.UnmarshalPER(r, enc); err != nil {
			return err
		}

		if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
			return err
		}

		*u = UES1APIDs{MMEUES1APID: mme, ENBUES1APID: enb, Pair: true}

		return nil
	default:
		var mme MMEUES1APID
		if err := mme.UnmarshalPER(r, enc); err != nil {
			return err
		}

		*u = UES1APIDs{MMEUES1APID: mme}

		return nil
	}
}

const ues1apIDsChoiceRootCount = 2

// UEContextReleaseCommand is the UE CONTEXT RELEASE COMMAND message (TS 36.413),
// sent by the MME to release a UE's S1 context.
type UEContextReleaseCommand struct {
	UES1APIDs UES1APIDs
	Cause     Cause
	unmodeledIEs
}

func (m *UEContextReleaseCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idUES1APIDs, crit: CriticalityReject, val: &m.UES1APIDs},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UEContextReleaseCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseUEContextReleaseCommand decodes the message from an initiatingMessage
// open-type payload.
func ParseUEContextReleaseCommand(value []byte) (*UEContextReleaseCommand, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: UEContextReleaseCommand preamble: %w", err)
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

	m := &UEContextReleaseCommand{}

	var seenIDs, seenCause bool

	for _, f := range fields {
		switch f.id {
		case idUES1APIDs:
			err = perIEDecode(f.value, &m.UES1APIDs)
			seenIDs = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UEContextReleaseCommand IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcUEContextRelease,
		ieCheck{idUES1APIDs, CriticalityReject, seenIDs},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// UEContextReleaseComplete is the UE CONTEXT RELEASE COMPLETE message (TS 36.413),
// sent by the eNB once the context is released.
type UEContextReleaseComplete struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	unmodeledIEs
}

func (m *UEContextReleaseComplete) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
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
func (m *UEContextReleaseComplete) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseUEContextReleaseComplete decodes the message from a successfulOutcome
// open-type payload.
func ParseUEContextReleaseComplete(value []byte) (*UEContextReleaseComplete, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: UEContextReleaseComplete preamble: %w", err)
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

	m := &UEContextReleaseComplete{}

	var seenMME, seenENB bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
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
			return nil, fmt.Errorf("s1ap: UEContextReleaseComplete IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcUEContextRelease,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// UEContextReleaseRequest is the UE CONTEXT RELEASE REQUEST message (TS 36.413),
// sent by the eNB to request release of a UE's S1 context (e.g. on
// radio-link failure or inactivity).
type UEContextReleaseRequest struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

func (m *UEContextReleaseRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UEContextReleaseRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextReleaseRequest,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseUEContextReleaseRequest decodes the message from an initiatingMessage
// open-type payload.
func ParseUEContextReleaseRequest(value []byte) (*UEContextReleaseRequest, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: UEContextReleaseRequest preamble: %w", err)
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

	m := &UEContextReleaseRequest{}

	var seenMME, seenENB, seenCause bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UEContextReleaseRequest IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcUEContextReleaseRequest,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}
