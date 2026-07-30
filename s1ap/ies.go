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

// Exactly one of val and raw is set; raw holds already-encoded open-type
// content, so an unmodeled IE round-trips verbatim.
type ieField struct {
	id   ProtocolIEID
	crit Criticality
	val  per.Marshaler
	raw  []byte
}

type rawIE struct {
	id    ProtocolIEID
	crit  Criticality
	value []byte
}

func (e rawIE) field() ieField {
	return ieField{id: e.id, crit: e.crit, raw: e.value}
}

// RawIE is a ProtocolIE-Field the message type does not model, with its value
// left as open-type bytes (TS 36.413).
type RawIE struct {
	ID          ProtocolIEID
	Criticality Criticality
	Value       []byte
}

// messageMeta is embedded in every message struct. It carries what the typed
// fields cannot: IEs this version does not model, and the abstract syntax
// errors that did not stop delivery.
type messageMeta struct {
	unknownIEs  []rawIE
	diagnostics Diagnostics
}

// preserve keeps an unmodeled IE so it survives a re-encode, up to a bound: a
// peer chooses both the count and the size of what we would retain.
func (u *messageMeta) preserve(f rawIE) {
	if len(u.unknownIEs) >= maxPreservedIEs {
		u.diagnostics.Truncated = true

		return
	}

	u.unknownIEs = append(u.unknownIEs, f)
}

// Diagnostics returns the abstract syntax errors found while decoding that
// TS 36.413 §10.3.4.2 and §10.3.5 let the receiver carry on past.
func (u messageMeta) Diagnostics() Diagnostics { return u.diagnostics }

// UnknownIEs returns, in wire order, the IEs this message type does not model.
func (u messageMeta) UnknownIEs() []RawIE {
	if len(u.unknownIEs) == 0 {
		return nil
	}

	out := make([]RawIE, len(u.unknownIEs))
	for i, e := range u.unknownIEs {
		out[i] = RawIE{ID: e.id, Criticality: e.crit, Value: e.value}
	}

	return out
}

// encodeIEContainer writes a ProtocolIE-Container (TS 36.413).
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
// ProtocolIE-SingleContainer (TS 36.413). ub is each list's own ASN.1 SIZE
// bound; that today's lists share 256 is coincidental.
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
// (TS 36.413). Each item is its own open type, so each gets a fresh reader.
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

// decodeIEContainer reads a ProtocolIE-Container in wire order, keeping every
// field including ids the caller does not model.
//
//nolint:unparam
func decodeIEContainer(r *per.Reader, enc per.Encoding) ([]rawIE, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("s1ap: IE container length: %w", err)
	}

	// A ProtocolIE-Field costs at least an id, a criticality and a non-empty
	// open type, so a count the remaining octets cannot hold is bogus and must
	// not be pre-allocated for.
	if maxBits := int64(r.Bits()); n > maxBits/minIEFieldBits {
		return nil, fmt.Errorf("s1ap: IE container declares %d IEs in %d bits", n, maxBits)
	}

	fields := make([]rawIE, 0, n)

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
