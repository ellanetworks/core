// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

// Builder assembles a NAS octet string information element by information element,
// for tests that must produce *any* message — well-formed or deliberately malformed.
// It performs no validation: that is the job of the decoder under test. Every method
// returns the receiver so calls chain.
//
// It is the shared adversarial-construction primitive for both RATs (TS 24.007 L3
// framing is common to 24.501/5GS and 24.301/EPS); the fgs and eps packages provide
// header-seeded constructors (Build/BuildRaw) on top of it.
//
// Deliberate-corruption support: reorder or omit IEs by choosing what to append,
// declare a length that does not match the value (LVn/LVEn/TLVn/TLVEn), append an
// unknown IEI, repeat an IE, Truncate the message mid-field, or add trailing garbage
// with Raw.
type Builder struct {
	w Writer
}

// NewBuilder returns an empty Builder. Prefer the fgs/eps header-seeded constructors
// unless you are hand-crafting the header octets too.
func NewBuilder() *Builder { return &Builder{} }

// U8 appends one octet (a mandatory field octet, a header octet, or trailing garbage).
func (b *Builder) U8(v uint8) *Builder {
	b.w.U8(v)
	return b
}

// U16 appends a big-endian 16-bit value.
func (b *Builder) U16(v uint16) *Builder {
	b.w.U16(v)
	return b
}

// Raw appends octets verbatim: a type-2/V value, a mandatory field, or trailing garbage.
func (b *Builder) Raw(p ...byte) *Builder {
	b.w.Raw(p)
	return b
}

// LV appends a value prefixed by a correct 1-octet length (an LV field / type-4 value part).
func (b *Builder) LV(v []byte) *Builder {
	b.w.U8(uint8(len(v)))
	b.w.Raw(v)

	return b
}

// LVE appends a value prefixed by a correct 2-octet length (an LV-E field / type-6 value part).
func (b *Builder) LVE(v []byte) *Builder {
	b.w.U16(uint16(len(v)))
	b.w.Raw(v)

	return b
}

// LVn appends a 1-octet declared length then v verbatim; declaredLen need not equal
// len(v), so a decoder can be exercised against an over- or under-declared field.
func (b *Builder) LVn(declaredLen uint8, v []byte) *Builder {
	b.w.U8(declaredLen)
	b.w.Raw(v)

	return b
}

// LVEn is LVn with a 2-octet declared length.
func (b *Builder) LVEn(declaredLen uint16, v []byte) *Builder {
	b.w.U16(declaredLen)
	b.w.Raw(v)

	return b
}

// TV1 appends a type-1 IE: the IEI in the high nibble and a 4-bit value in the low
// nibble, packed into one octet (TS 24.007 §11.2.4).
func (b *Builder) TV1(iei, val uint8) *Builder {
	b.w.U8(iei&0xF0 | val&0x0F)
	return b
}

// TV appends a type-3 IE: a full-octet IEI followed by a fixed-length value.
func (b *Builder) TV(iei uint8, v []byte) *Builder {
	b.w.U8(iei)
	b.w.Raw(v)

	return b
}

// TLV appends a type-4 IE: IEI, a correct 1-octet length, then the value.
func (b *Builder) TLV(iei uint8, v []byte) *Builder {
	b.w.U8(iei)
	return b.LV(v)
}

// TLVE appends a type-6 IE: IEI, a correct 2-octet length, then the value.
func (b *Builder) TLVE(iei uint8, v []byte) *Builder {
	b.w.U8(iei)
	return b.LVE(v)
}

// TLVn appends a type-4 IE with an arbitrary (possibly wrong) declared length octet.
func (b *Builder) TLVn(iei, declaredLen uint8, v []byte) *Builder {
	b.w.U8(iei)
	return b.LVn(declaredLen, v)
}

// TLVEn appends a type-6 IE with an arbitrary (possibly wrong) declared 2-octet length.
func (b *Builder) TLVEn(iei uint8, declaredLen uint16, v []byte) *Builder {
	b.w.U8(iei)
	return b.LVEn(declaredLen, v)
}

// Truncate keeps only the first n octets built so far, modelling a message cut short
// mid-IE. n is clamped to the current length.
func (b *Builder) Truncate(n int) *Builder {
	b.w.Truncate(n)
	return b
}

// Len is the number of octets assembled so far.
func (b *Builder) Len() int { return b.w.Len() }

// Bytes returns a fresh copy of the assembled octets, so later mutation of the
// returned slice cannot corrupt a builder still in use.
func (b *Builder) Bytes() []byte {
	out := make([]byte, b.w.Len())
	copy(out, b.w.Bytes())

	return out
}
