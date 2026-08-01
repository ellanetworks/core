// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Retention bounds for attacker-controlled input: a peer may fill a container
// with as many IEs as the message carries, and both recording a diagnostic and
// keeping an unmodeled IE for re-encode outlive the decode that found them.
const (
	maxDiagnosticIEs = 64
	maxPreservedIEs  = 32
)

// minIEFieldBits is the smallest a ProtocolIE-Field can be on the wire: a
// 16-bit id, a 2-bit criticality, and an open type of at least one length
// octet and one content octet.
const minIEFieldBits = 34

// ieSpec is one row of a message's TS 38.413 §9.2/§9.3 IE table. Encode and
// decode both read it, so the two directions cannot disagree on an IE's
// criticality or presence.
type ieSpec[M any] struct {
	id       ProtocolIEID
	presence presence
	crit     Criticality

	// condition reports whether a conditional IE must be present in m. It is
	// set if and only if presence is presenceConditional.
	condition func(m *M) bool

	decode func(m *M, raw []byte, enc per.Encoding) error
	// encode reports false when the row's field is unset.
	encode func(m *M) (per.Marshaler, bool)
}

// required reports whether TS 38.413 §10.3.3 obliges the sender to include the
// IE in this message.
func (s ieSpec[M]) required(m *M) bool {
	switch s.presence {
	case presenceMandatory:
		return true
	case presenceConditional:
		return s.condition != nil && s.condition(m)
	case presenceOptional:
		return false
	default:
		return false
	}
}

// deliverable reports whether a receiver still processes the message when the
// IE is absent (TS 38.413 §10.3.5). Only reject criticality stops delivery,
// which is what lets a required reject IE be a value type.
func (s ieSpec[M]) deliverable() bool { return s.crit != CriticalityReject }

func (u *messageMeta) meta() *messageMeta { return u }

// message is satisfied by a pointer to any message struct.
type message interface {
	meta() *messageMeta
}

// encodeMessageBody writes the SEQUENCE extension bit, then every present IE
// in table order, then any IE preserved verbatim from a previous decode.
//
// A required IE that is unset is an error: TS 38.413 §10.3.3 binds the sender
// even where §10.3.5 lets a receiver carry on without it.
func encodeMessageBody[M any, PM interface {
	*M
	message
}](w *per.Writer, enc per.Encoding, procedure ProcedureCode, table []ieSpec[M], m PM,
) error {
	w.WriteBit(false)

	fields := make([]ieField, 0, len(table))

	for _, spec := range table {
		val, ok := spec.encode((*M)(m))
		required := spec.required((*M)(m))

		if !ok {
			if required {
				return fmt.Errorf("ngap: %s: required IE %s is not set", procedure, spec.id)
			}

			continue
		}

		// §10.3.3: a conditional IE is carried only while its condition holds.
		if spec.presence == presenceConditional && !required {
			return fmt.Errorf("ngap: %s: conditional IE %s is set but its condition does not hold", procedure, spec.id)
		}

		fields = append(fields, ieField{id: spec.id, crit: spec.crit, val: val})
	}

	for _, e := range m.meta().unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// parseMessageBody decodes an IE container against its table, applying the
// abstract syntax error handling of TS 38.413 §10.3.4.2, §10.3.5 and §10.3.6.
//
// It returns no message when the procedure must be rejected. Otherwise every
// value-typed field of the result holds what the peer sent, and anything the
// receiver may carry on without is reported through Diagnostics.
func parseMessageBody[M any, PM interface {
	*M
	message
}](procedure ProcedureCode, trigger TriggeringMessage, table []ieSpec[M], value []byte,
) (PM, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, &TransferSyntaxError{Procedure: procedure, Err: fmt.Errorf("preamble: %w", err)}
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, &TransferSyntaxError{Procedure: procedure, Err: err}
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, &TransferSyntaxError{Procedure: procedure, Err: err}
		}
	}

	reject := func(cause int, ies []CriticalityDiagnosticsIEItem) error {
		return &AbstractSyntaxError{
			Procedure: procedure,
			Trigger:   trigger,
			Cause:     Cause{Group: CauseGroupProtocol, Value: cause},
			IEs:       ies,
			decoded:   modeledIEs(table, fields),
		}
	}

	m := PM(new(M))
	meta := m.meta()
	seen := make(map[ProtocolIEID]bool, len(table))
	lastIdx := -1

	for _, f := range fields {
		idx, spec, ok := lookupIESpec(table, f.id)
		if !ok {
			// §10.3.4.2: a not-comprehended IE marked reject stops the
			// procedure; the rest are reported and carried past.
			if f.crit == CriticalityReject {
				return nil, reject(CauseProtocolAbstractSyntaxErrorReject,
					[]CriticalityDiagnosticsIEItem{{
						IECriticality: f.crit,
						IEID:          f.id,
						TypeOfError:   TypeOfErrorNotUnderstood,
					}})
			}

			meta.diagnostics.record(f.id, f.crit, TypeOfErrorNotUnderstood)
			meta.preserve(f)

			continue
		}

		// §10.3.6: too many occurrences, or out of the order §9.2/§9.3 defines.
		// Only IEs this version knows are considered, so the unknown ids
		// handled above do not advance the cursor.
		if idx <= lastIdx {
			return nil, reject(CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage, nil)
		}

		lastIdx = idx

		if err := spec.decode((*M)(m), f.value, enc); err != nil {
			return nil, &TransferSyntaxError{
				Procedure: procedure,
				Err:       fmt.Errorf("IE %s: %w", f.id, err),
			}
		}

		seen[f.id] = true
	}

	// §10.3.5: absence is judged per IE, by the criticality §9.2/§9.3 assigns
	// it in this message.
	var missingReject []CriticalityDiagnosticsIEItem

	for i := range table {
		spec := table[i]
		required := spec.required((*M)(m))

		if seen[spec.id] {
			// §10.3.6: a conditional IE carried when its condition is false.
			if spec.presence == presenceConditional && !required {
				return nil, reject(CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage, nil)
			}

			continue
		}

		if !required {
			continue
		}

		if spec.deliverable() {
			meta.diagnostics.record(spec.id, spec.crit, TypeOfErrorMissing)

			continue
		}

		missingReject = append(missingReject, CriticalityDiagnosticsIEItem{
			IECriticality: spec.crit,
			IEID:          spec.id,
			TypeOfError:   TypeOfErrorMissing,
		})
	}

	if len(missingReject) > 0 {
		return nil, reject(CauseProtocolAbstractSyntaxErrorReject, missingReject)
	}

	return m, nil
}

func lookupIESpec[M any](table []ieSpec[M], id ProtocolIEID) (int, ieSpec[M], bool) {
	for i := range table {
		if table[i].id == id {
			return i, table[i], true
		}
	}

	return -1, ieSpec[M]{}, false
}

// modeledIEs keeps the first occurrence of each IE the table names, which is
// what addressing an unsuccessful outcome needs, and bounds the result by the
// table length, so the peer does not choose its size.
func modeledIEs[M any](table []ieSpec[M], fields []rawIE) []RawIE {
	var out []RawIE

	seen := make(map[ProtocolIEID]bool, len(table))

	for _, f := range fields {
		if seen[f.id] {
			continue
		}

		if _, _, ok := lookupIESpec(table, f.id); !ok {
			continue
		}

		seen[f.id] = true

		out = append(out, RawIE{ID: f.id, Criticality: f.crit, Value: f.value})
	}

	return out
}
