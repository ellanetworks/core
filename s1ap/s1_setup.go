// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
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

func (m *S1SetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idGlobalENBID, crit: CriticalityReject, val: &m.GlobalENBID},
	}

	if m.ENBName != "" {
		name := m.ENBName

		fields = append(fields, ieField{id: idENBname, crit: CriticalityIgnore, val: Name(name)})
	}

	fields = append(fields,
		ieField{id: idSupportedTAs, crit: CriticalityReject, val: m.SupportedTAs},
		ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, val: m.DefaultPagingDRX},
	)

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *S1SetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseS1SetupRequest decodes an S1SetupRequest from the open-type payload of an
// initiatingMessage (the InitiatingMessage.Value).
func ParseS1SetupRequest(value []byte) (*S1SetupRequest, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: S1SetupRequest preamble: %w", err)
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

	m := &S1SetupRequest{}

	var seenGlobalENBID, seenSupportedTAs bool

	for _, f := range fields {
		switch f.id {
		case idGlobalENBID:
			err = perIEDecode(f.value, &m.GlobalENBID)
			seenGlobalENBID = true
		case idENBname:
			var n Name

			err = perIEDecode(f.value, &n)
			m.ENBName = string(n)
		case idSupportedTAs:
			err = perIEDecode(f.value, &m.SupportedTAs)
			seenSupportedTAs = true
		case idDefaultPagingDRX:
			err = perIEDecode(f.value, &m.DefaultPagingDRX)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: S1SetupRequest IE %d: %w", f.id, err)
		}
	}

	// Default Paging DRX is ignore-criticality (TS 36.413 §9.1.8.4): a missing one
	// is ignored rather than rejected (§10.3.5).
	var missing []ProtocolIEID
	if !seenGlobalENBID {
		missing = append(missing, idGlobalENBID)
	}

	if !seenSupportedTAs {
		missing = append(missing, idSupportedTAs)
	}

	if len(missing) > 0 {
		return nil, &MissingMandatoryIEsError{Procedure: ProcS1Setup, IEs: missing}
	}

	return m, nil
}

// MissingMandatoryIEsError reports the reject-criticality mandatory IEs absent from a
// decoded initiating message, so the receiver can reject the procedure and name them
// in Criticality Diagnostics (TS 36.413 §10.3.5).
type MissingMandatoryIEsError struct {
	Procedure ProcedureCode
	IEs       []ProtocolIEID
}

func (e *MissingMandatoryIEsError) Error() string {
	return fmt.Sprintf("s1ap: procedure %d missing %d mandatory reject-criticality IE(s)", e.Procedure, len(e.IEs))
}
