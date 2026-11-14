// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
)

// A peer chooses how many IEs to send, and both survive the decode that found
// them.
const (
	maxDiagnosticIEs = 64
	maxPreservedIEs  = 32
)

// 16-bit id + 2-bit criticality + an open type of at least one length and one
// content octet.
const minIEFieldBits = 34

// One row of a message's TS 38.413 §9.2/§9.3 IE table.
type ieSpec[M any] struct {
	id       ProtocolIEID
	presence presence
	crit     Criticality

	// Set if and only if presence is presenceConditional.
	condition func(m *M) bool

	decode func(m *M, raw []byte, enc per.Encoding) error
	// encode reports false when the row's field is unset.
	encode func(m *M) (per.Marshaler, bool)
}

// TS 38.413 §9.1.1 Presence.
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

// §10.3.5: only reject criticality stops an absent IE from being delivered,
// which is what lets a required reject IE be a value type.
func (s ieSpec[M]) deliverable() bool { return s.crit != CriticalityReject }

func (u *messageMeta) meta() *messageMeta { return u }

type message interface {
	meta() *messageMeta
}

// An unset required IE is an error: TS 38.413 §9.1.1 obliges the sender even
// where §10.3.5 lets a receiver carry on without it.
func encodeMessageBody[M any, PM interface {
	*M
	message
}](w *per.Writer, enc per.Encoding, procedure ProcedureCode, table []ieSpec[M], m PM,
) error {
	w.WriteBit(false)

	typed := make([]*ieField, len(table))

	for i, spec := range table {
		val, ok := spec.encode((*M)(m))
		required := spec.required((*M)(m))

		if !ok {
			if required {
				return fmt.Errorf("ngap: %s: required IE %s is not set", procedure, spec.id)
			}

			continue
		}

		// §9.1.1: a conditional IE is carried only while its condition holds.
		if spec.presence == presenceConditional && !required {
			return fmt.Errorf("ngap: %s: conditional IE %s is set but its condition does not hold", procedure, spec.id)
		}

		typed[i] = &ieField{id: spec.id, crit: spec.crit, val: val}
	}

	return encodeIEContainer(w, enc, orderFields(table, typed, m.meta()))
}

func orderFields[M any](table []ieSpec[M], typed []*ieField, meta *messageMeta) []ieField {
	fields := make([]ieField, 0, len(table)+len(meta.unknownIEs))

	if meta.wire == nil {
		for _, f := range typed {
			if f != nil {
				fields = append(fields, *f)
			}
		}

		for _, e := range meta.unknownIEs {
			fields = append(fields, e.field())
		}

		return fields
	}

	onWire := make([]bool, len(table))

	for _, s := range meta.wire {
		if idx, _, ok := lookupIESpec(table, s.id); ok && s.unknown < 0 {
			onWire[idx] = true
		}
	}

	next := 0

	flush := func(upto int) {
		for ; next < upto; next++ {
			if typed[next] != nil && !onWire[next] {
				fields = append(fields, *typed[next])
			}
		}
	}

	for _, s := range meta.wire {
		if s.unknown >= 0 {
			fields = append(fields, meta.unknownIEs[s.unknown].field())

			continue
		}

		idx, _, ok := lookupIESpec(table, s.id)
		if !ok {
			continue
		}

		flush(idx)

		if f := typed[idx]; f != nil {
			fields = append(fields, ieField{id: f.id, crit: s.crit, val: f.val})
		}

		next = idx + 1
	}

	flush(len(table))

	return fields
}

// Applies the abstract syntax error handling of TS 38.413 §10.3.4.2, §10.3.5 and
// §10.3.6. Returns no message when the procedure must be rejected.
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

	// Collected for one report naming them all (§10.3.4.2), truncated at the
	// maxnoofErrors the list is bounded by so the report stays encodable.
	var notUnderstood []CriticalityDiagnosticsIEItem

	reportNotUnderstood := func(id ProtocolIEID, crit Criticality) {
		if len(notUnderstood) >= maxnoofErrors {
			return
		}

		notUnderstood = append(notUnderstood, CriticalityDiagnosticsIEItem{
			IECriticality: crit,
			IEID:          id,
			TypeOfError:   TypeOfErrorNotUnderstood,
		})
	}

	for _, f := range fields {
		idx, spec, ok := lookupIESpec(table, f.id)
		if !ok {
			// §10.3.4.2: reject stops the procedure, the rest are carried
			// past. §10.3.1 has the receiver read on either way, so the scan
			// continues and reports every offender together.
			if f.crit == CriticalityReject {
				reportNotUnderstood(f.id, f.crit)

				continue
			}

			meta.diagnostics.record(f.id, f.crit, TypeOfErrorNotUnderstood)
			meta.preserve(f)

			continue
		}

		// §10.3.6: out of the order §9.2/§9.3 defines, or carried twice. Unknown
		// ids do not advance the cursor.
		if idx <= lastIdx {
			// Abandoning the scan must not drop what it already collected.
			return nil, reject(CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage, notUnderstood)
		}

		lastIdx = idx

		if err := spec.decode((*M)(m), f.value, enc); err != nil {
			// §10.3.1 cases 1, 2 and 6 are handled on criticality, not by
			// abandoning the message.
			if errors.Is(err, errNotComprehended) {
				// §10.3.2 handles the item on its own criticality, which for an
				// unmodeled extension is the extension's, not this IE's.
				id, crit := f.id, f.crit

				var item *notComprehendedIE
				if errors.As(err, &item) {
					id, crit = item.ID, item.Crit
				}

				if crit == CriticalityReject {
					reportNotUnderstood(id, crit)

					continue
				}

				meta.diagnostics.record(id, crit, TypeOfErrorNotUnderstood)

				continue
			}

			return nil, &TransferSyntaxError{
				Procedure: procedure,
				Err:       fmt.Errorf("IE %s: %w", f.id, err),
			}
		}

		seen[f.id] = true

		meta.wire = append(meta.wire, wireSlot{id: f.id, crit: f.crit, unknown: -1})
	}

	if wireMatchesTable(table, meta.wire) {
		meta.wire = nil
	}

	if len(notUnderstood) > 0 {
		return nil, reject(CauseProtocolAbstractSyntaxErrorReject, notUnderstood)
	}

	// §10.3.5: absence is judged per IE, by the criticality §9.2/§9.3 assigns it.
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

func wireMatchesTable[M any](table []ieSpec[M], wire []wireSlot) bool {
	for _, s := range wire {
		if s.unknown >= 0 {
			return false
		}

		if _, spec, ok := lookupIESpec(table, s.id); !ok || spec.crit != s.crit {
			return false
		}
	}

	return true
}

func lookupIESpec[M any](table []ieSpec[M], id ProtocolIEID) (int, ieSpec[M], bool) {
	for i := range table {
		if table[i].id == id {
			return i, table[i], true
		}
	}

	return -1, ieSpec[M]{}, false
}

// First occurrence only, bounded by the table length so the peer does not
// choose the size.
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
