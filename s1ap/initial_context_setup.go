// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// InitialContextSetupRequest is the INITIAL CONTEXT SETUP REQUEST message
// (TS 36.413 §9.1.4.1), sent by the MME to set up the UE context and default
// E-RAB(s). Unmodeled IEs are preserved.
type InitialContextSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate UEAggregateMaximumBitRate
	ERABToBeSetup             []ERABToBeSetupItemCtxtSUReq
	UESecurityCapabilities    UESecurityCapabilities
	SecurityKey               SecurityKey
	// UERadioCapability is the optional UE Radio Capability IE (TS 36.413
	// §9.1.4.1); when set, the eNB reuses it instead of re-fetching it from the
	// UE over the air (TS 23.401 §5.11.2).
	UERadioCapability []byte

	unmodeledIEs
}

func (m *InitialContextSetupRequest) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, enc: m.UEAggregateMaximumBitRate.encode},
		{id: idERABToBeSetupListCtxtSUReq, crit: CriticalityReject, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABToBeSetupItemCtxtSUReq, CriticalityReject, encoderList(m.ERABToBeSetup))
		}},
		{id: idUESecurityCapabilities, crit: CriticalityReject, enc: m.UESecurityCapabilities.encode},
		{id: idSecurityKey, crit: CriticalityReject, enc: m.SecurityKey.encode},
	}

	if len(m.UERadioCapability) > 0 {
		fields = append(fields, ieField{id: idUERadioCapability, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return w.WriteOctetString(m.UERadioCapability, 0, aper.Unbounded, false)
		}})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupRequest decodes the message from an initiatingMessage
// open-type payload.
func ParseInitialContextSetupRequest(value []byte) (*InitialContextSetupRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialContextSetupRequest preamble: %w", err)
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

	m := &InitialContextSetupRequest{}

	var seenMME, seenENB, seenAMBR, seenERAB, seenSec, seenKey bool

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
			m.UEAggregateMaximumBitRate, err = decodeUEAggregateMaximumBitRate(sub)
			seenAMBR = true
		case idERABToBeSetupListCtxtSUReq:
			m.ERABToBeSetup, err = decodeERABToBeSetupList(sub)
			seenERAB = true
		case idUESecurityCapabilities:
			m.UESecurityCapabilities, err = decodeUESecurityCapabilities(sub)
			seenSec = true
		case idSecurityKey:
			m.SecurityKey, err = decodeSecurityKey(sub)
			seenKey = true
		case idUERadioCapability:
			m.UERadioCapability, err = sub.ReadOctetString(0, aper.Unbounded, false)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialContextSetupRequest IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenAMBR || !seenERAB || !seenSec || !seenKey {
		return nil, fmt.Errorf("s1ap: InitialContextSetupRequest missing mandatory IE")
	}

	return m, nil
}

func decodeERABToBeSetupList(r *aper.Reader) ([]ERABToBeSetupItemCtxtSUReq, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABToBeSetupItemCtxtSUReq)
}

// InitialContextSetupResponse is the INITIAL CONTEXT SETUP RESPONSE message
// (TS 36.413 §9.1.4.2), sent by the eNB once the E-RAB(s) are set up.
type InitialContextSetupResponse struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	ERABSetup              []ERABSetupItemCtxtSURes
	ERABFailedToSetup      []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *InitialContextSetupResponse) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
		{id: idERABSetupListCtxtSURes, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABSetupItemCtxtSURes, CriticalityIgnore, encoderList(m.ERABSetup))
		}},
	}

	if len(m.ERABFailedToSetup) > 0 {
		fields = append(fields, ieField{id: idERABFailedToSetupListCtxtSU, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABFailedToSetup))
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
func (m *InitialContextSetupResponse) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupResponse decodes the message from a successfulOutcome
// open-type payload.
func ParseInitialContextSetupResponse(value []byte) (*InitialContextSetupResponse, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialContextSetupResponse preamble: %w", err)
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

	m := &InitialContextSetupResponse{}

	var seenMME, seenENB, seenSetup bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idERABSetupListCtxtSURes:
			m.ERABSetup, err = decodeERABSetupList(sub)
			seenSetup = true
		case idERABFailedToSetupListCtxtSU:
			m.ERABFailedToSetup, err = decodeERABItemList(sub)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialContextSetupResponse IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenSetup {
		return nil, fmt.Errorf("s1ap: InitialContextSetupResponse missing mandatory IE")
	}

	return m, nil
}

func decodeERABSetupList(r *aper.Reader) ([]ERABSetupItemCtxtSURes, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABSetupItemCtxtSURes)
}

func decodeERABItemList(r *aper.Reader) ([]ERABItem, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABItem)
}

// InitialContextSetupFailure is the INITIAL CONTEXT SETUP FAILURE message
// (TS 36.413 §9.1.4.3).
type InitialContextSetupFailure struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *InitialContextSetupFailure) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
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
func (m *InitialContextSetupFailure) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupFailure decodes the message from an
// unsuccessfulOutcome open-type payload.
func ParseInitialContextSetupFailure(value []byte) (*InitialContextSetupFailure, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialContextSetupFailure preamble: %w", err)
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

	m := &InitialContextSetupFailure{}

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
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialContextSetupFailure IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCause {
		return nil, fmt.Errorf("s1ap: InitialContextSetupFailure missing mandatory IE")
	}

	return m, nil
}
