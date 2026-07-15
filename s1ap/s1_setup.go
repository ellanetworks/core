// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// S1SetupRequest is the S1 SETUP REQUEST message (TS 36.413). An empty
// ENBName means the optional eNBname IE is absent. IEs that are not modeled are
// preserved in unknownIEs so the message round-trips.
type S1SetupRequest struct {
	GlobalENBID      GlobalENBID
	ENBName          string
	SupportedTAs     SupportedTAs
	DefaultPagingDRX PagingDRX

	unmodeledIEs
}

func (m *S1SetupRequest) encodeBody(w *aper.Writer) error {
	// S1SetupRequest ::= SEQUENCE { protocolIEs, ... }: extensible, no optional
	// root fields, no extension additions emitted.
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idGlobalENBID, crit: CriticalityReject, enc: m.GlobalENBID.encode},
	}

	if m.ENBName != "" {
		name := m.ENBName

		fields = append(fields, ieField{id: idENBname, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeName(w, name)
		}})
	}

	fields = append(fields,
		ieField{id: idSupportedTAs, crit: CriticalityReject, enc: m.SupportedTAs.encode},
		ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, enc: m.DefaultPagingDRX.encode},
	)

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *S1SetupRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseS1SetupRequest decodes an S1SetupRequest from the open-type payload of an
// initiatingMessage (the InitiatingMessage.Value).
func ParseS1SetupRequest(value []byte) (*S1SetupRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: S1SetupRequest preamble: %w", err)
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

	m := &S1SetupRequest{}

	var seenGlobalENBID, seenSupportedTAs, seenPagingDRX bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idGlobalENBID:
			m.GlobalENBID, err = decodeGlobalENBID(sub)
			seenGlobalENBID = true
		case idENBname:
			m.ENBName, err = decodeName(sub)
		case idSupportedTAs:
			m.SupportedTAs, err = decodeSupportedTAs(sub)
			seenSupportedTAs = true
		case idDefaultPagingDRX:
			m.DefaultPagingDRX, err = decodePagingDRX(sub)
			seenPagingDRX = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: S1SetupRequest IE %d: %w", f.id, err)
		}
	}

	if !seenGlobalENBID || !seenSupportedTAs || !seenPagingDRX {
		return nil, fmt.Errorf("s1ap: S1SetupRequest missing mandatory IE")
	}

	return m, nil
}
