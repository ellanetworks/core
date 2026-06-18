// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import "fmt"

// IEFormat classifies a NAS L3 information element's encoding (TS 24.007
// §11.2.1.1). It tells the walker how to delimit an IE in a message's variable
// part.
type IEFormat uint8

const (
	// IETV1 is a type-1 (TV, ½ octet) or type-2 (T) IE: a single octet whose IEI
	// is the high nibble (≥ 0x80) and whose value, if any, is the low nibble. It
	// needs no table entry — the walker recognises it by the high bit.
	IETV1 IEFormat = iota
	// IETV3 is a type-3 (TV) IE: a full-octet IEI (< 0x80) followed by a
	// fixed-length value with no length octet. Its length must be declared.
	IETV3
	// IETLV is a type-4 IE: a full-octet IEI, a one-octet length, then the value.
	IETLV
	// IETLVE is a type-6 IE: a full-octet IEI, a two-octet length, then the value.
	IETLVE
)

// OptionalIE describes one IE of a message's variable part: its IEI, its format,
// and (for IETV3 only) its fixed value length. A message declares these in a
// table transcribed from the IE list in its spec definition (TS 24.301 §8.2.x).
type OptionalIE struct {
	IEI    uint8
	Format IEFormat
	Len    int // value length, IETV3 only
}

// WalkOptionalIEs walks the variable (optional) part of a NAS message, delimiting
// each IE with table and passing its IEI and value to emit (TS 24.007 §11.2).
//
// A type-1/2 IE (IEI ≥ 0x80) is always one octet — emit receives the high-nibble
// IEI and the low nibble as the value — and needs no table entry. A full-octet
// IEI (< 0x80) must appear in table to be delimited; one that does not cannot be
// length-delimited safely (its format is not derivable from the IEI), so the walk
// stops and returns the undecoded remainder for the caller to keep verbatim
// rather than guessing a length and corrupting the parse. All reads are bounded
// by r, so a hostile length octet cannot over-read.
func WalkOptionalIEs(r *Reader, table []OptionalIE, emit func(iei uint8, value []byte) error) (rest []byte, err error) {
	for r.Remaining() > 0 {
		iei, err := r.PeekU8()
		if err != nil {
			return nil, err
		}

		if iei >= 0x80 {
			if _, err := r.U8(); err != nil {
				return nil, err
			}

			if err := emit(iei&0xF0, []byte{iei & 0x0F}); err != nil {
				return nil, err
			}

			continue
		}

		def, ok := lookupIE(table, iei)
		if !ok {
			return r.Bytes(r.Remaining())
		}

		if _, err := r.U8(); err != nil {
			return nil, err
		}

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
			return nil, err
		}

		if err := emit(iei, value); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func lookupIE(table []OptionalIE, iei uint8) (OptionalIE, bool) {
	for _, d := range table {
		if d.IEI == iei {
			return d, true
		}
	}

	return OptionalIE{}, false
}
