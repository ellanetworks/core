// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// UES1APIDs is the UE-S1AP-IDs CHOICE (TS 36.413 §9.2.3.x): either the
// UE-S1AP-ID-pair (both identities, the form an MME sends) or a bare
// MME-UE-S1AP-ID. Pair selects which alternative.
type UES1APIDs struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Pair        bool
}

const ues1apIDsChoiceRootCount = 2

func (u UES1APIDs) encode(w *aper.Writer) error {
	if !u.Pair {
		if err := w.WriteChoiceIndex(1, ues1apIDsChoiceRootCount, true, false); err != nil {
			return err
		}

		return u.MMEUES1APID.encode(w)
	}

	if err := w.WriteChoiceIndex(0, ues1apIDsChoiceRootCount, true, false); err != nil {
		return err
	}

	// UE-S1AP-ID-pair ::= SEQUENCE { mME-UE-S1AP-ID, eNB-UE-S1AP-ID,
	// iE-Extensions OPTIONAL } (extensible).
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := u.MMEUES1APID.encode(w); err != nil {
		return err
	}

	return u.ENBUES1APID.encode(w)
}

func decodeUES1APIDs(r *aper.Reader) (UES1APIDs, error) {
	idx, isExt, err := r.ReadChoiceIndex(ues1apIDsChoiceRootCount, true)
	if err != nil {
		return UES1APIDs{}, err
	}

	if isExt {
		return UES1APIDs{}, fmt.Errorf("s1ap: UE-S1AP-IDs extension alternative unsupported")
	}

	switch idx {
	case 0:
		extPresent, opt, err := r.ReadSequencePreamble(true, 1)
		if err != nil {
			return UES1APIDs{}, err
		}

		mme, err := decodeMMEUES1APID(r)
		if err != nil {
			return UES1APIDs{}, err
		}

		enb, err := decodeENBUES1APID(r)
		if err != nil {
			return UES1APIDs{}, err
		}

		if opt[0] {
			if err := skipExtensionContainer(r); err != nil {
				return UES1APIDs{}, err
			}
		}

		if extPresent {
			if err := r.SkipExtensionAdditions(); err != nil {
				return UES1APIDs{}, err
			}
		}

		return UES1APIDs{MMEUES1APID: mme, ENBUES1APID: enb, Pair: true}, nil
	case 1:
		mme, err := decodeMMEUES1APID(r)
		if err != nil {
			return UES1APIDs{}, err
		}

		return UES1APIDs{MMEUES1APID: mme}, nil
	default:
		return UES1APIDs{}, fmt.Errorf("s1ap: unexpected UE-S1AP-IDs choice index %d", idx)
	}
}

// UEContextReleaseCommand is the UE CONTEXT RELEASE COMMAND message (TS 36.413
// §9.1.4.6), sent by the MME to release a UE's S1 context.
type UEContextReleaseCommand struct {
	UES1APIDs UES1APIDs
	Cause     Cause
	unmodeledIEs
}

func (m *UEContextReleaseCommand) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idUES1APIDs, crit: CriticalityReject, enc: m.UES1APIDs.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UEContextReleaseCommand) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseUEContextReleaseCommand decodes the message from an initiatingMessage
// open-type payload.
func ParseUEContextReleaseCommand(value []byte) (*UEContextReleaseCommand, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: UEContextReleaseCommand preamble: %w", err)
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

	m := &UEContextReleaseCommand{}

	var seenIDs, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idUES1APIDs:
			m.UES1APIDs, err = decodeUES1APIDs(sub)
			seenIDs = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UEContextReleaseCommand IE %d: %w", f.id, err)
		}
	}

	if !seenIDs || !seenCause {
		return nil, fmt.Errorf("s1ap: UEContextReleaseCommand missing mandatory IE")
	}

	return m, nil
}

// UEContextReleaseComplete is the UE CONTEXT RELEASE COMPLETE message (TS 36.413
// §9.1.4.7), sent by the eNB once the context is released.
type UEContextReleaseComplete struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *UEContextReleaseComplete) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
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
func (m *UEContextReleaseComplete) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseUEContextReleaseComplete decodes the message from a successfulOutcome
// open-type payload.
func ParseUEContextReleaseComplete(value []byte) (*UEContextReleaseComplete, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: UEContextReleaseComplete preamble: %w", err)
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

	m := &UEContextReleaseComplete{}

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
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UEContextReleaseComplete IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB {
		return nil, fmt.Errorf("s1ap: UEContextReleaseComplete missing mandatory IE")
	}

	return m, nil
}

// UEContextReleaseRequest is the UE CONTEXT RELEASE REQUEST message (TS 36.413
// §9.1.4.5), sent by the eNB to request release of a UE's S1 context (e.g. on
// radio-link failure or inactivity).
type UEContextReleaseRequest struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

func (m *UEContextReleaseRequest) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UEContextReleaseRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextReleaseRequest,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseUEContextReleaseRequest decodes the message from an initiatingMessage
// open-type payload.
func ParseUEContextReleaseRequest(value []byte) (*UEContextReleaseRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: UEContextReleaseRequest preamble: %w", err)
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

	m := &UEContextReleaseRequest{}

	var seenMME, seenENB, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UEContextReleaseRequest IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCause {
		return nil, fmt.Errorf("s1ap: UEContextReleaseRequest missing mandatory IE")
	}

	return m, nil
}
