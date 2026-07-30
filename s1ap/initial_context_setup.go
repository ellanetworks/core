// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// InitialContextSetupRequest is the INITIAL CONTEXT SETUP REQUEST message
// (TS 36.413), sent by the MME to set up the UE context and default
// E-RAB(s). Unmodeled IEs are preserved.
type InitialContextSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate UEAggregateMaximumBitRate
	ERABToBeSetup             []ERABToBeSetupItemCtxtSUReq
	UESecurityCapabilities    UESecurityCapabilities
	SecurityKey               SecurityKey
	// UERadioCapability is the optional UE Radio Capability IE (TS 36.413);
	// when set, the eNB reuses it and skips re-fetching it from the
	// UE over the air (TS 23.401).
	UERadioCapability []byte

	unmodeledIEs
}

func (m *InitialContextSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, val: &m.UEAggregateMaximumBitRate},
		{id: idERABToBeSetupListCtxtSUReq, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSetupItemCtxtSUReq, CriticalityReject, m.ERABToBeSetup)
		})},
		{id: idUESecurityCapabilities, crit: CriticalityReject, val: &m.UESecurityCapabilities},
		{id: idSecurityKey, crit: CriticalityReject, val: &m.SecurityKey},
	}

	if len(m.UERadioCapability) > 0 {
		fields = append(fields, ieField{id: idUERadioCapability, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapability)
		})})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupRequest decodes the message from an initiatingMessage
// open-type payload.
func ParseInitialContextSetupRequest(value []byte) (*InitialContextSetupRequest, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialContextSetupRequest preamble: %w", err)
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

	m := &InitialContextSetupRequest{}

	var seenMME, seenENB, seenAMBR, seenERAB, seenSec, seenKey bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			err = perIEDecode(f.value, &m.UEAggregateMaximumBitRate)
			seenAMBR = true
		case idERABToBeSetupListCtxtSUReq:
			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemCtxtSUReq](per.NewReader(f.value), enc, maxnoofERABs)
			seenERAB = true
		case idUESecurityCapabilities:
			err = perIEDecode(f.value, &m.UESecurityCapabilities)
			seenSec = true
		case idSecurityKey:
			err = perIEDecode(f.value, &m.SecurityKey)
			seenKey = true
		case idUERadioCapability:
			m.UERadioCapability, err = per.DecodeOctetString(per.NewReader(f.value), enc, 0, 0, true, false, false)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialContextSetupRequest IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcInitialContextSetup,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idUEAggregateMaximumBitrate, CriticalityReject, seenAMBR},
		ieCheck{idERABToBeSetupListCtxtSUReq, CriticalityReject, seenERAB},
		ieCheck{idUESecurityCapabilities, CriticalityReject, seenSec},
		ieCheck{idSecurityKey, CriticalityReject, seenKey},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// InitialContextSetupResponse is the INITIAL CONTEXT SETUP RESPONSE message
// (TS 36.413), sent by the eNB once the E-RAB(s) are set up.
type InitialContextSetupResponse struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	ERABSetup              []ERABSetupItemCtxtSURes
	ERABFailedToSetup      []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *InitialContextSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
		{id: idERABSetupListCtxtSURes, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABSetupItemCtxtSURes, CriticalityIgnore, m.ERABSetup)
		})},
	}

	if len(m.ERABFailedToSetup) > 0 {
		fields = append(fields, ieField{id: idERABFailedToSetupListCtxtSU, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToSetup)
		})})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: &d})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupResponse decodes the message from a successfulOutcome
// open-type payload.
func ParseInitialContextSetupResponse(value []byte) (*InitialContextSetupResponse, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialContextSetupResponse preamble: %w", err)
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

	m := &InitialContextSetupResponse{}

	var seenMME, seenENB, seenSetup bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABSetupListCtxtSURes:
			m.ERABSetup, err = decodeItemList[ERABSetupItemCtxtSURes](per.NewReader(f.value), enc, maxnoofERABs)
			seenSetup = true
		case idERABFailedToSetupListCtxtSU:
			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialContextSetupResponse IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcInitialContextSetup,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
		ieCheck{idERABSetupListCtxtSURes, CriticalityIgnore, seenSetup},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// InitialContextSetupFailure is the INITIAL CONTEXT SETUP FAILURE message
// (TS 36.413).
type InitialContextSetupFailure struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *InitialContextSetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: &d})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupFailure decodes the message from an
// unsuccessfulOutcome open-type payload.
func ParseInitialContextSetupFailure(value []byte) (*InitialContextSetupFailure, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialContextSetupFailure preamble: %w", err)
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

	m := &InitialContextSetupFailure{}

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
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialContextSetupFailure IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcInitialContextSetup,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}
