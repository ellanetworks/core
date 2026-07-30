// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// S1SetupRequest is the S1 SETUP REQUEST message (TS 36.413 §9.1.8.4). A nil
// ENBName means the optional eNBname IE is absent; a nil DefaultPagingDRX means
// the mandatory-but-ignore-criticality IE was absent (§10.3.5). IEs that are
// not modeled are preserved in unknownIEs so the message round-trips.
type S1SetupRequest struct {
	GlobalENBID      GlobalENBID
	ENBName          *string
	SupportedTAs     SupportedTAs
	DefaultPagingDRX *PagingDRX

	unmodeledIEs
}

func (m *S1SetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idGlobalENBID, crit: CriticalityReject, val: &m.GlobalENBID},
	}

	if m.ENBName != nil {
		fields = append(fields, ieField{id: idENBname, crit: CriticalityIgnore, val: Name(*m.ENBName)})
	}

	fields = append(fields, ieField{id: idSupportedTAs, crit: CriticalityReject, val: m.SupportedTAs})

	if m.DefaultPagingDRX != nil {
		fields = append(fields, ieField{id: idDefaultPagingDRX, crit: CriticalityIgnore, val: *m.DefaultPagingDRX})
	}

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

	var seenGlobalENBID, seenSupportedTAs, seenPagingDRX bool

	for _, f := range fields {
		switch f.id {
		case idGlobalENBID:
			err = perIEDecode(f.value, &m.GlobalENBID)
			seenGlobalENBID = true
		case idENBname:
			var n Name

			if err = perIEDecode(f.value, &n); err == nil {
				name := string(n)
				m.ENBName = &name
			}
		case idSupportedTAs:
			err = perIEDecode(f.value, &m.SupportedTAs)
			seenSupportedTAs = true
		case idDefaultPagingDRX:
			var drx PagingDRX

			if err = perIEDecode(f.value, &drx); err == nil {
				m.DefaultPagingDRX = &drx
			}

			seenPagingDRX = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: S1SetupRequest IE %d: %w", f.id, err)
		}
	}

	// §9.1.8.4: Default Paging DRX is mandatory but ignore-criticality, so its
	// absence is reported without rejecting the procedure (§10.3.5).
	if err := requireIEs(ProcS1Setup,
		ieCheck{idGlobalENBID, CriticalityReject, seenGlobalENBID},
		ieCheck{idSupportedTAs, CriticalityReject, seenSupportedTAs},
		ieCheck{idDefaultPagingDRX, CriticalityIgnore, seenPagingDRX},
	); err != nil {
		return nil, err
	}

	return m, nil
}
