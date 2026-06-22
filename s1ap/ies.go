// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// Container size bounds (TS 36.413 §9.3, S1AP-Constants).
const (
	maxProtocolIEs        = 65535
	maxProtocolExtensions = 65535
)

// ieField is one ProtocolIE-Field to encode: its id, criticality, and a
// function that writes the value body. The engine wraps the body as an open
// type.
type ieField struct {
	id   ProtocolIEID
	crit Criticality
	enc  func(*aper.Writer) error
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
	return ieField{
		id:   e.id,
		crit: e.crit,
		enc: func(w *aper.Writer) error {
			w.WriteOctets(e.value)
			return nil
		},
	}
}

// RawIE is an exported view of a ProtocolIE-Field the message layer does not
// model: its id, criticality, and raw open-type value bytes (TS 36.413 §9.3).
// It lets callers surface IEs present on the wire that the typed fields omit,
// rather than dropping them.
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

// encodeIEContainer writes a ProtocolIE-Container (TS 36.413 §9.3): the field
// count as a constrained length, then each ProtocolIE-Field as
// { id, criticality, value-as-open-type } in order.
func encodeIEContainer(w *aper.Writer, fields []ieField) error {
	if len(fields) > maxProtocolIEs {
		return fmt.Errorf("s1ap: %d IEs exceed maxProtocolIEs", len(fields))
	}

	if err := w.WriteConstrainedLength(len(fields), 0, maxProtocolIEs); err != nil {
		return err
	}

	for _, f := range fields {
		if err := w.WriteConstrainedInt(int64(f.id), 0, maxProtocolIEs); err != nil {
			return err
		}

		if err := w.WriteEnum(int(f.crit), criticalityRootCount, false, false); err != nil {
			return err
		}

		var vw aper.Writer

		if f.enc != nil {
			if err := f.enc(&vw); err != nil {
				return fmt.Errorf("s1ap: encode IE %d: %w", f.id, err)
			}
		}

		if err := w.WriteOpenType(vw.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// maxnoofERABs bounds the E-RAB SEQUENCE-OF lists (TS 36.413 §9.3).
const maxnoofERABs = 256

// encodeSingleContainerList writes a SEQUENCE (SIZE(1..ub)) OF
// ProtocolIE-SingleContainer: a constrained count, then each item as a
// { id, criticality, value-as-open-type } field with the fixed item id and
// criticality. Used by the E-RAB and TAI lists (TS 36.413 §9.3). ub is each
// list's ASN.1 SIZE bound; it is coincidental that the current lists share 256.
//
//nolint:unparam
func encodeSingleContainerList(w *aper.Writer, ub int, id ProtocolIEID, crit Criticality, items []func(*aper.Writer) error) error {
	if len(items) < 1 || len(items) > ub {
		return fmt.Errorf("s1ap: single-container list length %d outside [1, %d]", len(items), ub)
	}

	if err := w.WriteConstrainedLength(len(items), 1, ub); err != nil {
		return err
	}

	for _, enc := range items {
		if err := w.WriteConstrainedInt(int64(id), 0, maxProtocolIEs); err != nil {
			return err
		}

		if err := w.WriteEnum(int(crit), criticalityRootCount, false, false); err != nil {
			return err
		}

		var vw aper.Writer

		if err := enc(&vw); err != nil {
			return err
		}

		if err := w.WriteOpenType(vw.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// decodeSingleContainerList reads a SEQUENCE (SIZE(1..ub)) OF
// ProtocolIE-SingleContainer, returning the open-type value bytes of each item
// for the caller to decode. ub is each list's ASN.1 SIZE bound; it is
// coincidental that the current lists share 256.
//
//nolint:unparam
func decodeSingleContainerList(r *aper.Reader, ub int) ([][]byte, error) {
	n, err := r.ReadConstrainedLength(1, ub)
	if err != nil {
		return nil, err
	}

	out := make([][]byte, 0, min(n, 16))

	for i := 0; i < n; i++ {
		if _, err := r.ReadConstrainedInt(0, maxProtocolIEs); err != nil {
			return nil, err
		}

		if _, _, err := r.ReadEnum(criticalityRootCount, false); err != nil {
			return nil, err
		}

		val, err := r.ReadOpenType()
		if err != nil {
			return nil, err
		}

		out = append(out, val)
	}

	return out, nil
}

// decodeItemList reads a SEQUENCE (SIZE(1..ub)) OF ProtocolIE-SingleContainer
// (TS 36.413 §9.3). Each item is its own APER open type, so dec is handed a
// fresh reader over that item's octets — element decoders therefore take an
// *aper.Reader like every other decoder, and the open-type boundary lives here
// rather than being re-stated at each call site.
func decodeItemList[T any](r *aper.Reader, ub int, dec func(*aper.Reader) (T, error)) ([]T, error) {
	raw, err := decodeSingleContainerList(r, ub)
	if err != nil {
		return nil, err
	}

	out := make([]T, 0, len(raw))

	for _, b := range raw {
		it, err := dec(aper.NewReader(b))
		if err != nil {
			return nil, err
		}

		out = append(out, it)
	}

	return out, nil
}

// encoderList adapts a slice of encodable items to the []func(*aper.Writer) error
// that encodeSingleContainerList consumes.
func encoderList[T interface{ encode(*aper.Writer) error }](items []T) []func(*aper.Writer) error {
	out := make([]func(*aper.Writer) error, len(items))
	for i := range items {
		out[i] = items[i].encode
	}

	return out
}

// skipSequenceExtensions steps over a SEQUENCE's optional iE-Extensions
// (ProtocolExtensionContainer) and extension additions when they are present but
// not modeled (TS 36.413 §9.3).
func skipSequenceExtensions(r *aper.Reader, extContainer, extAdditions bool) error {
	if extContainer {
		if err := skipExtensionContainer(r); err != nil {
			return err
		}
	}

	if extAdditions {
		return r.SkipExtensionAdditions()
	}

	return nil
}

// decodeIEContainer reads a ProtocolIE-Container into the fields in wire order,
// preserving every field (including ids the caller does not model) for dispatch
// by id. The field slice grows by append, so a corrupt count cannot force a
// large allocation: the loop fails as soon as the input is exhausted.
func decodeIEContainer(r *aper.Reader) ([]rawIE, error) {
	n, err := r.ReadConstrainedLength(0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("s1ap: IE container length: %w", err)
	}

	return readContainerFields(r, n)
}

// skipExtensionContainer consumes a ProtocolExtensionContainer (TS 36.413 §9.3)
// and discards it. The container differs from a ProtocolIE-Container only in its
// SIZE(1..maxProtocolExtensions) bound; its fields decode identically. Used to
// step over the optional iE-Extensions of IEs whose extensions are not modeled.
func skipExtensionContainer(r *aper.Reader) error {
	n, err := r.ReadConstrainedLength(1, maxProtocolExtensions)
	if err != nil {
		return fmt.Errorf("s1ap: extension container length: %w", err)
	}

	_, err = readContainerFields(r, n)

	return err
}

// readContainerFields reads n protocol-IE/extension fields in wire order. The
// slice grows by append, so a corrupt count cannot force a large allocation:
// the loop fails as soon as the input is exhausted.
func readContainerFields(r *aper.Reader, n int) ([]rawIE, error) {
	var fields []rawIE

	for i := 0; i < n; i++ {
		id, err := r.ReadConstrainedInt(0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("s1ap: IE %d id: %w", i, err)
		}

		crit, _, err := r.ReadEnum(criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("s1ap: IE %d criticality: %w", i, err)
		}

		val, err := r.ReadOpenType()
		if err != nil {
			return nil, fmt.Errorf("s1ap: IE %d value: %w", i, err)
		}

		fields = append(fields, rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val})
	}

	return fields, nil
}
