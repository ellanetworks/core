// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Container size bounds (TS 36.413, S1AP-Constants).
const (
	maxProtocolIEs        = 65535
	maxProtocolExtensions = 65535
)

// ieField is one ProtocolIE-Field to encode: its id, criticality, and the
// value. The engine wraps the value as an open type. Exactly one of val and
// raw is set; raw carries the pre-encoded open-type content of an IE decoded
// from the wire but not modeled, so it round-trips verbatim.
type ieField struct {
	id   ProtocolIEID
	crit Criticality
	val  per.Marshaler
	raw  []byte
}

// rawIE is a decoded ProtocolIE-Field: id, criticality, and the raw open-type
// value bytes. The message layer decodes the bytes for ids it models and
// preserves the rest so they survive a re-encode.
type rawIE struct {
	id    ProtocolIEID
	crit  Criticality
	value []byte
}

// field returns an ieField that re-emits this decoded IE verbatim, so unknown
// IEs round-trip.
func (e rawIE) field() ieField {
	return ieField{id: e.id, crit: e.crit, raw: e.value}
}

// RawIE is an exported view of a ProtocolIE-Field the message layer does not
// model: its id, criticality, and raw open-type value bytes (TS 36.413).
// It lets callers surface IEs present on the wire that the typed fields omit.
type RawIE struct {
	ID          ProtocolIEID
	Criticality Criticality
	Value       []byte
}

// unmodeledIEs is embedded in every message struct. It holds the ProtocolIEs the
// message type does not model so they round-trip on re-encode and callers can
// surface them. The field is unexported (the engine appends to it during decode
// and re-emits it on encode); UnknownIEs exposes a read-only view.
type unmodeledIEs struct {
	unknownIEs []rawIE
}

// UnhandledCriticalIEs returns, in wire order, the ids of the ProtocolIEs this
// message type does not model that arrived marked with reject criticality.
//
// TS 36.413 §10.3.4.2 requires a receiver that does not comprehend such an IE
// to reject the procedure and report it. The codec records them instead of
// deciding: only the receiving node knows the procedure and whether it has a
// message with which to report the unsuccessful outcome. A caller that does
// understand one of these ids may disregard it and carry on.
//
// The IEs themselves remain available from UnknownIEs and are re-emitted on
// encode, so a message still round-trips with everything that arrived.
func (u unmodeledIEs) UnhandledCriticalIEs() []ProtocolIEID {
	var out []ProtocolIEID

	for _, e := range u.unknownIEs {
		if e.crit == CriticalityReject {
			out = append(out, e.id)
		}
	}

	return out
}

// UnknownIEs returns, in wire order, the ProtocolIEs present on the wire that
// this message type does not model, as raw {id, criticality, value} triples.
func (u unmodeledIEs) UnknownIEs() []RawIE {
	if len(u.unknownIEs) == 0 {
		return nil
	}

	out := make([]RawIE, len(u.unknownIEs))
	for i, e := range u.unknownIEs {
		out[i] = RawIE{ID: e.id, Criticality: e.crit, Value: e.value}
	}

	return out
}

// encodeIEContainer writes a ProtocolIE-Container (TS 36.413): the field
// count as a constrained length, then each ProtocolIE-Field as
// { id, criticality, value-as-open-type } in order.
func encodeIEContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	if len(fields) > maxProtocolIEs {
		return fmt.Errorf("s1ap: %d IEs exceed maxProtocolIEs", len(fields))
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, maxProtocolIEs, int64(len(fields))); err != nil {
		return err
	}

	for _, f := range fields {
		if err := encodeContainerField(w, enc, f); err != nil {
			return err
		}
	}

	return nil
}

func encodeContainerField(w *per.Writer, enc per.Encoding, f ieField) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, maxProtocolIEs, int64(f.id)); err != nil {
		return err
	}

	if err := per.EncodeEnumerated(w, enc, criticalityRootCount, false, int64(f.crit)); err != nil {
		return err
	}

	if f.raw != nil {
		return per.EncodeOpenTypeBytes(w, enc, f.raw)
	}

	if f.val == nil {
		return fmt.Errorf("s1ap: IE %d has no value", f.id)
	}

	if err := per.EncodeOpenType(w, enc, f.val); err != nil {
		return fmt.Errorf("s1ap: encode IE %d: %w", f.id, err)
	}

	return nil
}

// maxnoofERABs bounds the E-RAB SEQUENCE-OF lists (TS 36.413).
const maxnoofERABs = 256

// encodeSingleContainerList writes a SEQUENCE (SIZE(1..ub)) OF
// ProtocolIE-SingleContainer: a constrained count, then each item as a
// { id, criticality, value-as-open-type } field with the fixed item id and
// criticality. Used by the E-RAB and TAI lists (TS 36.413). ub is each
// list's ASN.1 SIZE bound; it is coincidental that the current lists share 256.
//
//nolint:unparam
func encodeSingleContainerList[T any](w *per.Writer, enc per.Encoding, ub int64, id ProtocolIEID, crit Criticality, items []T) error {
	if len(items) < 1 || int64(len(items)) > ub {
		return fmt.Errorf("s1ap: single-container list length %d outside [1, %d]", len(items), ub)
	}

	off := 0

	return per.EncodeLength(w, enc, 1, ub, true, int64(len(items)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			m, ok := any(&items[i]).(per.Marshaler)
			if !ok {
				return fmt.Errorf("s1ap: %T does not implement per.Marshaler", items[i])
			}

			if err := encodeContainerField(w, enc, ieField{id: id, crit: crit, val: m}); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

// decodeItemList reads a SEQUENCE (SIZE(1..ub)) OF ProtocolIE-SingleContainer
// (TS 36.413). Each item is its own APER open type, decoded with a fresh
// reader over that item's octets, so the open-type boundary stays here, not at
// each call site.
//
//nolint:unparam
func decodeItemList[T any](r *per.Reader, enc per.Encoding, ub int64) ([]T, error) {
	var items []T

	err := per.DecodeLength(r, enc, 1, ub, true, func(count int64) error {
		for i := int64(0); i < count; i++ {
			if _, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs); err != nil {
				return err
			}

			if _, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false); err != nil {
				return err
			}

			val, err := per.DecodeOpenTypeBytes(r, enc)
			if err != nil {
				return err
			}

			var item T

			u, ok := any(&item).(per.Unmarshaler)
			if !ok {
				return fmt.Errorf("s1ap: %T does not implement per.Unmarshaler", item)
			}

			if err := u.UnmarshalPER(per.NewReader(val), enc); err != nil {
				return err
			}

			items = append(items, item)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

// decodeIEContainer reads a ProtocolIE-Container into the fields in wire order,
// preserving every field (including ids the caller does not model) for dispatch
// by id.
//
//nolint:unparam
func decodeIEContainer(r *per.Reader, enc per.Encoding) ([]rawIE, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("s1ap: IE container length: %w", err)
	}

	var fields []rawIE

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("s1ap: IE %d id: %w", i, err)
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("s1ap: IE %d criticality: %w", i, err)
		}

		val, err := per.DecodeOpenTypeBytes(r, enc)
		if err != nil {
			return nil, fmt.Errorf("s1ap: IE %d value: %w", i, err)
		}

		fields = append(fields, rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val})
	}

	return fields, nil
}
