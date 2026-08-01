// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Container size bounds (TS 38.413, NGAP-Constants).
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
// left as open-type bytes (TS 38.413).
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
// TS 38.413 §10.3.4.2 and §10.3.5 let the receiver carry on past.
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

// encodeIEContainer writes a ProtocolIE-Container (TS 38.413).
func encodeIEContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	if len(fields) > maxProtocolIEs {
		return fmt.Errorf("ngap: %d IEs exceed maxProtocolIEs", len(fields))
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
		return fmt.Errorf("ngap: IE %d has no value", f.id)
	}

	if err := per.EncodeOpenType(w, enc, f.val); err != nil {
		return fmt.Errorf("ngap: encode IE %d: %w", f.id, err)
	}

	return nil
}

// decodeChoiceExtension reads the ProtocolIE-SingleContainer that closes an
// NGAP CHOICE (TS 38.413 §9.3) and reports the alternative as unsupported.
//
// Where S1AP marks a CHOICE extensible, NGAP gives it a choice-Extensions
// alternative carrying one open IE. A peer selecting it has sent a value this
// version cannot represent, so decoding says so by id instead of returning a
// zero value the caller would read as a root alternative.
func decodeChoiceExtension(r *per.Reader, enc per.Encoding, choice string) error {
	id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return fmt.Errorf("ngap: %s choice-Extensions id: %w", choice, err)
	}

	if _, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false); err != nil {
		return fmt.Errorf("ngap: %s choice-Extensions criticality: %w", choice, err)
	}

	if err := per.SkipOpenType(r, enc); err != nil {
		return fmt.Errorf("ngap: %s choice-Extensions value: %w", choice, err)
	}

	return fmt.Errorf("ngap: unsupported %s alternative %s", choice, ProtocolIEID(id))
}

// decodeIEContainer reads a ProtocolIE-Container in wire order, keeping every
// field including ids the caller does not model.
//
//nolint:unparam
func decodeIEContainer(r *per.Reader, enc per.Encoding) ([]rawIE, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("ngap: IE container length: %w", err)
	}

	// A ProtocolIE-Field costs at least an id, a criticality and a non-empty
	// open type, so a count the remaining octets cannot hold is bogus and must
	// not be pre-allocated for.
	if maxBits := int64(r.Bits()); n > maxBits/minIEFieldBits {
		return nil, fmt.Errorf("ngap: IE container declares %d IEs in %d bits", n, maxBits)
	}

	fields := make([]rawIE, 0, n)

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("ngap: IE %d id: %w", i, err)
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("ngap: IE %d criticality: %w", i, err)
		}

		val, err := per.DecodeOpenTypeBytes(r, enc)
		if err != nil {
			return nil, fmt.Errorf("ngap: IE %d value: %w", i, err)
		}

		fields = append(fields, rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val})
	}

	return fields, nil
}
