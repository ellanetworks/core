// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"fmt"
)

// IEFormat classifies a NAS L3 information element's encoding (TS 24.007).
// It tells the walker how to delimit an IE in a message's variable
// part.
type IEFormat uint8

const (
	// IEInvalid is the zero value, which names no encoding. Reserving it makes a
	// RawIE whose Format was never set fail loudly instead of being written as
	// some other format's shape.
	IEInvalid IEFormat = iota
	// IETV1 is a type-1 (TV, ½ octet) or type-2 (T) IE: a single octet whose IEI
	// is the high nibble (≥ 0x80) and whose value, if any, is the low nibble. It
	// needs no table entry — the walker recognises it by the high bit.
	IETV1
	// IETV3 is a type-3 (TV) IE: a full-octet IEI (< 0x80) followed by a
	// fixed-length value with no length octet. Its length must be declared.
	IETV3
	// IETLV is a type-4 IE: a full-octet IEI, a one-octet length, then the value.
	IETLV
	// IETLVE is a type-6 IE: a full-octet IEI, a two-octet length, then the value.
	IETLVE
	// IETruncated is not an encoding but an element whose declared length ran past
	// the end of the message. Value holds every octet after the IEI, so the
	// element re-emits exactly as it arrived. TS 24.501 §7.7.1 and §7.7.3.1 have
	// the receiver treat it as not present and carry on; nothing follows it, so
	// the walk ends there.
	IETruncated
)

func (f IEFormat) String() string {
	switch f {
	case IETV1:
		return "type 1/2"
	case IETV3:
		return "type 3"
	case IETLV:
		return "type 4"
	case IETLVE:
		return "type 6"
	case IETruncated:
		return "truncated"
	default:
		return "invalid"
	}
}

// OptionalIE describes one IE of a message's variable part: its IEI, its format,
// and (for IETV3 only) its fixed value length. A message declares these in a
// table transcribed from the IE list in its spec definition (TS 24.301).
type OptionalIE struct {
	IEI    uint8
	Format IEFormat
	Len    int // value length, IETV3 only

	// Name is the element's name in this message, for error messages. An IEI is
	// message-scoped, so the name belongs beside the framing rather than in a
	// package-wide table.
	Name string

	// Critical marks an element whose decode failure must fail the whole message
	// rather than leave the field absent. TS 24.501 §7.7.1 and TS 24.301 §7.7.1
	// would have a receiver treat a syntactically incorrect optional element as
	// not present, and for most elements that is right; for the ones a security
	// decision reads — the capabilities replayed for bidding-down detection
	// (TS 33.501 §6.7.2, TS 33.401 §7.2.4.4), and the containers that prove a
	// message echoes what the UE sent — a silently absent value is worse than a
	// rejected message.
	//
	// It lives on the table entry because an IEI is message-scoped: EPS 0x58 is
	// the UE network capability in EMM and the ESM cause in ESM, and 5GS 0x71 is
	// the NAS message container in 5GMM and the extended CAG information list in
	// REGISTRATION ACCEPT.
	Critical bool
}

// RawIE is an optional information element a message does not model, kept in the
// form it arrived so the message re-encodes byte-for-byte. TS 24.501 §7.6.1 and
// TS 24.301 §7.6.1 require a receiver to ignore an element it does not
// comprehend; keeping it means ignoring it without losing it.
type RawIE struct {
	IEI    uint8
	Format IEFormat
	Value  []byte

	// After is the IEI of the last element the message did model before this one
	// arrived, or 0 if none did. Re-encoding puts the element back after that
	// anchor, which reproduces the order it arrived in even when a sender
	// interleaves elements the message does not model with ones it does. It is
	// always 0 for an IETruncated element, which can only be re-emitted last.
	//
	// Elements sharing an IEI are the exception: they anchor together, to that
	// IEI when the message accepted one of them and to the first one's anchor
	// otherwise. Anchoring each where it happened to arrive would let a re-encode
	// separate them, and the two would swap places on every round trip.
	After uint8
}

// UnknownIE selects the rule the walker applies to a full-octet IEI absent from
// the table. TS 24.007 §11.2.4 gives one to each generation: a receiver assumes
// an unknown IE is type 1, 2, 4 or 6 — never type 3 — and tells type 4 from
// type 6 by the IEI alone. The two differ only in how wide the type-6 range is.
//
// Because type 3 is not in that list, a full-octet TV IE is not self-delimiting
// under either rule: its length lives in the spec's IE table and nowhere on the
// wire. Every such IE a message can carry must therefore appear in the walker
// table even when the message does not model it, or it is mis-skipped as a TLV
// and the walk desynchronizes.
type UnknownIE uint8

const (
	// UnknownIESkip5GS applies the 5GMM/5GSM rule: bits 7-5 of the IEI all set
	// (0x70-0x7F) mark a type-6 TLV-E, any other combination a type-4 TLV.
	UnknownIESkip5GS UnknownIE = iota
	// UnknownIESkipEPS applies the EMM/ESM rule: bits 7-4 of the IEI all set
	// (0x78-0x7F) mark a type-6 TLV-E, any other combination a type-4 TLV. The
	// range is narrower than the 5GS one, and 3GPP has left 0x70-0x77 unassigned
	// in EPS so that the difference stays invisible on the wire.
	UnknownIESkipEPS
)

// tlvEMask returns the IEI mask whose bits, when all set, mark a TLV-E under
// this rule (TS 24.007 §11.2.4).
func (u UnknownIE) tlvEMask() uint8 {
	if u == UnknownIESkipEPS {
		return 0x78
	}

	return 0x70
}

// Walker walks the variable (optional) part of a NAS message (TS 24.007),
// delimiting each element with Table and offering its IEI and value to Emit.
//
// Emit reports whether the message models that IEI; every element it declines,
// and every element absent from Table, is returned so the caller can carry it
// through re-encoding.
//
// A type-1/2 IE (IEI ≥ 0x80) is always one octet — Emit receives the high-nibble
// IEI and the low nibble as the value — and needs no table entry. All reads are
// bounded by the reader, so a hostile length octet cannot over-read.
//
// An element Emit rejects is treated as not present and its failure collected as
// an *IEError, per TS 24.501 §7.7.1 and TS 24.301 §7.7.1; the walk continues and
// the element is preserved so it still re-encodes. The returned error joins those
// failures and satisfies SoftOnly. Two things break that rule: an element whose
// table entry is Critical, whose failure is returned alone so no caller can act
// on a message with a silently absent security element; and a framing failure,
// which has desynchronised the walk and leaves the rest of the message
// unreadable.
//
// A repeated IEI is decoded once: TS 24.501 §7.6.3 and TS 24.301 §7.6.3 direct a
// receiver to handle the first occurrence and ignore the rest, so later ones are
// preserved without being offered to Emit.
type Walker struct {
	// Table describes the framing of the elements the message can carry.
	Table []OptionalIE
	// Unknown selects the rule for an IEI absent from Table.
	Unknown UnknownIE
	// Emit is offered each delimited element and reports whether the message
	// models that IEI.
	Emit func(iei uint8, value []byte) (modelled bool, err error)
}

// Walk runs the walk over r.
func (w Walker) Walk(r *Reader) (unrecognized []RawIE, err error) {
	return (&walker{table: w.Table, unknown: w.Unknown, emit: w.Emit}).run(r)
}

type walker struct {
	table   []OptionalIE
	unknown UnknownIE
	emit    func(iei uint8, value []byte) (bool, error)

	anchor   uint8
	seen     map[uint8]bool
	accepted map[uint8]bool
	soft     []error
}

// offer hands one delimited element to emit unless the message already claimed
// its IEI, and classifies whatever comes back. It reports whether the message
// modelled the element, and a non-nil error only when the failure is a hard one.
//
// An IEI is claimed by the first element the message recognises under it,
// whether or not that element decoded: TS 24.501 §7.6.3 and TS 24.301 §7.6.3
// give the first occurrence the field, and §7.7.1 makes a malformed one absent
// rather than a second chance for the sender.
func (w *walker) offer(iei uint8, format IEFormat, value []byte) (known bool, err error) {
	if w.seen[iei] {
		return false, nil
	}

	known, emitErr := w.emit(iei, value)
	if emitErr == nil {
		if known {
			w.claim(iei)

			if w.accepted == nil {
				w.accepted = make(map[uint8]bool)
			}

			w.accepted[iei] = true
		}

		return known, nil
	}

	w.claim(iei)

	if def, ok := lookupIE(w.table, iei); ok && def.Critical {
		return false, emitErr
	}

	w.soft = append(w.soft, &IEError{IEI: iei, Name: w.name(iei), Format: format, Raw: value, Err: emitErr})

	// Not known: the field stays absent, and preserving the element keeps its
	// octets so the message still re-encodes with everything it arrived with.
	return false, nil
}

// name is the element's name in this message, or "" when the table gives none.
func (w *walker) name(iei uint8) string {
	if def, ok := lookupIE(w.table, iei); ok {
		return def.Name
	}

	return ""
}

func (w *walker) claim(iei uint8) {
	if w.seen == nil {
		w.seen = make(map[uint8]bool)
	}

	w.seen[iei] = true
}

// result re-anchors the preserved elements that share an IEI, then returns the
// walk's soft failures joined.
//
// Two elements under one IEI have to stay together and in arrival order,
// wherever encoding places them: anchoring each to the element it happened to
// arrive behind would emit the two in one order and re-read them in the other,
// so they would trade places on every round trip. Where they belong depends on
// whether the message accepted one of them, which is only settled once the walk
// ends, since the accepted element may arrive after the preserved one:
//
//   - accepted: the preserved ones follow the modelled element, so they anchor to
//     their own IEI;
//   - not accepted: the IEI is the message's only as a preserved element, so they
//     all anchor where the first of them arrived.
//
// A truncated element is exempt: it re-emits last whatever its anchor.
func (w *walker) result(unrecognized []RawIE) ([]RawIE, error) {
	firstAnchor := make(map[uint8]uint8, len(unrecognized))

	for i := range unrecognized {
		ie := &unrecognized[i]

		switch {
		case ie.Format == IETruncated:
		case w.accepted[ie.IEI]:
			ie.After = ie.IEI
		default:
			if anchor, ok := firstAnchor[ie.IEI]; ok {
				ie.After = anchor
			} else {
				firstAnchor[ie.IEI] = ie.After
			}
		}
	}

	return unrecognized, errors.Join(w.soft...)
}

// preserveTruncated keeps an element whose declared length ran past the end of
// the message. It rewinds to just after the IEI and takes every remaining octet,
// so the element re-emits exactly as it arrived, and records the failure as soft:
// TS 24.501 §7.7.1 and TS 24.301 §7.7.1 have the receiver treat a syntactically
// incorrect optional element as not present, and §7.7.3.1 EXAMPLE 2 spells out
// this case. Nothing follows an element that overruns the buffer, so there is no
// desynchronized remainder to fear and the walk simply ends.
// A Critical element is the exception: its failure is returned alone, since a
// security decision must never read a silently absent value.
func (w *walker) preserveTruncated(r *Reader, afterIEI int, iei uint8, format IEFormat, cause error) (RawIE, error) {
	if def, ok := lookupIE(w.table, iei); ok && def.Critical {
		return RawIE{}, cause
	}

	r.pos = afterIEI

	rest, _ := r.Bytes(r.Remaining())

	w.soft = append(w.soft, &IEError{IEI: iei, Name: w.name(iei), Format: format, Raw: rest, Err: cause})

	return RawIE{IEI: iei, Format: IETruncated, Value: rest}, nil
}

func (w *walker) run(r *Reader) (unrecognized []RawIE, err error) {
	for r.Remaining() > 0 {
		iei, err := r.PeekU8()
		if err != nil {
			return nil, err
		}

		if iei >= 0x80 {
			if _, err := r.U8(); err != nil {
				return nil, err
			}

			value := []byte{iei & 0x0F}

			known, err := w.offer(iei&0xF0, IETV1, value)
			if err != nil {
				return nil, err
			}

			if known {
				w.anchor = iei & 0xF0
			} else {
				unrecognized = append(unrecognized, RawIE{IEI: iei & 0xF0, Format: IETV1, Value: value, After: w.anchor})
			}

			continue
		}

		def, ok := lookupIE(w.table, iei)
		if !ok {
			start := r.Offset()

			raw, err := skipSelfDelimiting(r, iei, w.unknown)
			if err != nil {
				if !errors.Is(err, ErrTruncated) {
					return nil, err
				}

				raw, hard := w.preserveTruncated(r, start+1, iei, IETLV, err)
				if hard != nil {
					return nil, hard
				}

				return w.result(append(unrecognized, raw))
			}

			raw.After = w.anchor
			unrecognized = append(unrecognized, raw)

			continue
		}

		if _, err := r.U8(); err != nil {
			return nil, err
		}

		afterIEI := r.Offset()

		var value []byte

		switch def.Format {
		case IETV3:
			value, err = r.Bytes(def.Len)
		case IETLV:
			value, err = r.LV()
		case IETLVE:
			value, err = r.LVE()
		default:
			return nil, fmt.Errorf("nas: IE %#x declared TV1 but has a full-octet IEI", iei)
		}

		if err != nil {
			if !errors.Is(err, ErrTruncated) {
				return nil, err
			}

			raw, hard := w.preserveTruncated(r, afterIEI, iei, def.Format, err)
			if hard != nil {
				return nil, hard
			}

			return w.result(append(unrecognized, raw))
		}

		known, err := w.offer(iei, def.Format, value)
		if err != nil {
			return nil, err
		}

		if known {
			w.anchor = iei
		} else {
			unrecognized = append(unrecognized, RawIE{IEI: iei, Format: def.Format, Value: value, After: w.anchor})
		}
	}

	return w.result(unrecognized)
}

// skipSelfDelimiting consumes an unrecognized full-octet IE by the format its
// IEI implies under unknown's rule (TS 24.007 §11.2.4) and returns it intact.
func skipSelfDelimiting(r *Reader, iei uint8, unknown UnknownIE) (RawIE, error) {
	if _, err := r.U8(); err != nil {
		return RawIE{}, err
	}

	format := IETLV
	read := r.LV

	if mask := unknown.tlvEMask(); iei&mask == mask {
		format = IETLVE
		read = r.LVE
	}

	value, err := read()
	if err != nil {
		return RawIE{}, err
	}

	return RawIE{IEI: iei, Format: format, Value: value}, nil
}

func lookupIE(table []OptionalIE, iei uint8) (OptionalIE, bool) {
	for _, d := range table {
		if d.IEI == iei {
			return d, true
		}
	}

	return OptionalIE{}, false
}

// OptionalWriter collects a message's optional information elements and emits
// them in the order they were added, which for a message encoder is the order
// 3GPP lists them in that message's definition. That order is per-message and is
// not ascending by IEI: TS 24.501 §8.3.2 lists the PDU SESSION ESTABLISHMENT
// ACCEPT elements 0x59, 0x29, 0x56, 0x22, 0x75, 0x78, 0x79, 0x7B, 0x25.
//
// Collecting rather than writing directly is what lets a message carry the
// elements it merely preserves alongside the ones it models: each preserved
// element goes back after the modelled element it followed on the wire, so a
// message re-encodes byte-for-byte even when the sender interleaved the two.
type OptionalWriter struct {
	items    []optionalItem
	preserve []RawIE
	tail     []RawIE
}

type optionalItem struct {
	iei    uint8
	format IEFormat
	value  []byte
}

// TV1 adds a type-1/2 IE: one octet whose IEI is the high nibble and whose
// value, if any, is the low nibble.
func (o *OptionalWriter) TV1(iei, value uint8) {
	o.add(iei&0xF0, IETV1, []byte{value & 0x0F})
}

// TV3 adds a type-3 IE: the IEI followed by a fixed-length value.
func (o *OptionalWriter) TV3(iei uint8, value []byte) { o.add(iei, IETV3, value) }

// TLV adds a type-4 IE: the IEI, a one-octet length, then the value.
func (o *OptionalWriter) TLV(iei uint8, value []byte) { o.add(iei, IETLV, value) }

// TLVE adds a type-6 IE: the IEI, a two-octet length, then the value.
func (o *OptionalWriter) TLVE(iei uint8, value []byte) { o.add(iei, IETLVE, value) }

// Raw adds elements preserved from decoding, each in the format it arrived.
//
// An IETruncated element is held back to the end. It overran the message, so it
// took every octet that followed it; re-emitting it anywhere but last would put
// those octets in front of elements that had preceded them, and the next parse
// would take a different number of them.
func (o *OptionalWriter) Raw(ies ...RawIE) {
	for _, ie := range ies {
		if ie.Format == IETruncated {
			o.tail = append(o.tail, ie)
			continue
		}

		o.preserve = append(o.preserve, ie)
	}
}

func (o *OptionalWriter) add(iei uint8, format IEFormat, value []byte) {
	o.items = append(o.items, optionalItem{iei: iei, format: format, value: value})
}

// WriteTo emits the collected elements onto w: those preserved from before any
// modelled element, then each modelled element followed by the preserved ones
// that trailed it, then any preserved element whose anchor this message did not
// emit.
func (o *OptionalWriter) WriteTo(w *Writer) {
	placed := make([]bool, len(o.preserve))

	o.writePreserved(w, 0, placed)

	for _, it := range o.items {
		o.writeItem(w, it)
		o.writePreserved(w, it.iei, placed)
	}

	for i, ie := range o.preserve {
		if !placed[i] {
			o.writeItem(w, optionalItem{iei: ie.IEI, format: ie.Format, value: ie.Value})
		}
	}

	for _, ie := range o.tail {
		o.writeItem(w, optionalItem{iei: ie.IEI, format: ie.Format, value: ie.Value})
	}
}

// writePreserved emits every preserved element anchored to iei that has not been
// emitted yet, in the order it arrived.
func (o *OptionalWriter) writePreserved(w *Writer, iei uint8, placed []bool) {
	for i, ie := range o.preserve {
		if placed[i] || ie.After != iei {
			continue
		}

		placed[i] = true

		o.writeItem(w, optionalItem{iei: ie.IEI, format: ie.Format, value: ie.Value})
	}
}

func (o *OptionalWriter) writeItem(w *Writer, it optionalItem) {
	switch it.format {
	case IETV1:
		// A type-1 IE's value is the low nibble; a type-2 IE has none. Both are
		// one octet, and a caller-built element with an empty value must not
		// index past it.
		var nibble uint8
		if len(it.value) > 0 {
			nibble = it.value[0] & 0x0F
		}

		w.U8(it.iei | nibble)
	case IETV3:
		w.U8(it.iei)
		w.Raw(it.value)
	case IETLV:
		w.TLV(it.iei, it.value)
	case IETLVE:
		w.TLVE(it.iei, it.value)
	case IETruncated:
		w.U8(it.iei)
		w.Raw(it.value)
	default:
		w.fail("preserved IE with no format")
	}
}
