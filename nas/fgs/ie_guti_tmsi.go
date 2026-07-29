// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// GUTI is a 5G-GUTI 5GS mobile identity (TS 24.501 §9.11.3.4), the 5GS
// counterpart of eps.GUTI.
type GUTI struct {
	PLMN        nas.PLMN
	AMFRegionID uint8
	AMFSetID    uint16  // 10 bits
	AMFPointer  uint8   // 6 bits
	TMSI        [4]byte // 5G-TMSI
}

// ParseGUTI decodes a 5G-GUTI 5GS mobile identity value (11 octets: type, PLMN,
// AMF Region ID, AMF Set ID + AMF Pointer, 5G-TMSI).
func ParseGUTI(b []byte) (GUTI, error) {
	if len(b) != 11 {
		return GUTI{}, fmt.Errorf("nas/fgs: 5G-GUTI is %d octets, want 11", len(b))
	}

	if typ := TypeOfIdentity(b[0]); typ != IdentityGUTI {
		return GUTI{}, fmt.Errorf("nas/fgs: mobile identity type %d is not 5G-GUTI", typ)
	}

	plmn, err := nas.ParsePLMN([3]byte{b[1], b[2], b[3]})
	if err != nil {
		return GUTI{}, err
	}

	return GUTI{
		PLMN:        plmn,
		AMFRegionID: b[4],
		AMFSetID:    uint16(b[5])<<2 | uint16(b[6])>>6,
		AMFPointer:  b[6] & 0x3f,
		TMSI:        [4]byte{b[7], b[8], b[9], b[10]},
	}, nil
}

// AppendBinary encodes the 5G-GUTI as its 11-octet 5GS mobile identity value.
// The encoding is appended to b.
func (g GUTI) AppendBinary(b []byte) ([]byte, error) {
	plmn, err := g.PLMN.Octets()
	if err != nil {
		return b, err
	}

	setID, pointer := packAMFSetIDPointer(g.AMFSetID, g.AMFPointer)

	b = append(b, 0xF0|uint8(IdentityGUTI)) // spare 1111, type-of-identity 010
	b = append(b, plmn[:]...)
	b = append(b, g.AMFRegionID, setID, pointer)
	b = append(b, g.TMSI[:]...)

	return b, nil
}

// MarshalBinary encodes the 5G-GUTI as its 5GS mobile identity value.
func (g GUTI) MarshalBinary() ([]byte, error) { return g.AppendBinary(nil) }

// unpackAMFSetIDPointer is the inverse of packAMFSetIDPointer.
func unpackAMFSetIDPointer(hi, lo byte) (setID uint16, pointer uint8) {
	return uint16(hi)<<2 | uint16(lo)>>6, lo & 0x3F
}

// packAMFSetIDPointer encodes the 10-bit AMF Set ID and 6-bit AMF Pointer into
// two octets (TS 24.501 §9.11.3.4).
func packAMFSetIDPointer(setID uint16, pointer uint8) (hi, lo byte) {
	sp := setID&0x3FF<<6 | uint16(pointer&0x3F)
	return byte(sp >> 8), byte(sp)
}

// AMFIdentifier is the 3-octet AMF identifier carried in a 5G-GUTI: the AMF
// Region ID, the 10-bit AMF Set ID, and the 6-bit AMF Pointer
// (TS 23.003 §2.10.1, TS 24.501 §9.11.3.4).
type AMFIdentifier struct {
	RegionID uint8
	SetID    uint16
	Pointer  uint8
}

// ParseAMFIdentifier decodes a 3-octet AMF identifier.
func ParseAMFIdentifier(v []byte) (AMFIdentifier, error) {
	if len(v) != 3 {
		return AMFIdentifier{}, fmt.Errorf("nas/fgs: AMF identifier: want 3 octets, got %d", len(v))
	}

	setID, pointer := unpackAMFSetIDPointer(v[1], v[2])

	return AMFIdentifier{RegionID: v[0], SetID: setID, Pointer: pointer}, nil
}

// AppendBinary encodes the AMF identifier onto b.
func (a AMFIdentifier) AppendBinary(b []byte) ([]byte, error) {
	hi, lo := packAMFSetIDPointer(a.SetID, a.Pointer)

	return append(b, a.RegionID, hi, lo), nil
}

// MarshalBinary encodes the AMF identifier.
func (a AMFIdentifier) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// STMSI is the 5G-S-TMSI carried as a type-4 5GS mobile identity (TS 24.501
// §9.11.3.4): a 10-bit AMF Set ID, a 6-bit AMF Pointer, and the 4-octet 5G-TMSI.
type STMSI struct {
	AMFSetID   uint16
	AMFPointer uint8
	TMSI       [4]byte
}

// ParseSTMSI decodes a 5G-S-TMSI mobile identity value (7 octets: type,
// AMF Set ID/Pointer, 5G-TMSI).
func ParseSTMSI(b []byte) (STMSI, error) {
	if len(b) != 7 {
		return STMSI{}, fmt.Errorf("nas/fgs: 5G-S-TMSI: want 7 octets, got %d", len(b))
	}

	if typ := TypeOfIdentity(b[0] & 0x07); typ != IdentitySTMSI {
		return STMSI{}, fmt.Errorf("nas/fgs: mobile identity type %d is not 5G-S-TMSI", typ)
	}

	out := STMSI{
		AMFSetID:   uint16(b[1])<<2 | uint16(b[2])>>6,
		AMFPointer: b[2] & 0x3F,
	}
	copy(out.TMSI[:], b[3:7])

	return out, nil
}

// AppendBinary encodes the 5G-S-TMSI as a 7-octet type-4 5GS mobile identity value.
// The encoding is appended to b.
func (s STMSI) AppendBinary(b []byte) ([]byte, error) {
	setID, pointer := packAMFSetIDPointer(s.AMFSetID, s.AMFPointer)

	// Bits 5-8 of octet 4 are coded as "1111" (TS 24.501 table 9.11.3.4.1).
	b = append(b, 0xF0|uint8(IdentitySTMSI), setID, pointer)
	b = append(b, s.TMSI[:]...)

	return b, nil
}

// MarshalBinary encodes the 5G-S-TMSI as its 5GS mobile identity value.
func (s STMSI) MarshalBinary() ([]byte, error) { return s.AppendBinary(nil) }

func (g GUTI) String() string {
	return fmt.Sprintf("%s-%02x-%03x-%02x-%x", g.PLMN, g.AMFRegionID, g.AMFSetID, g.AMFPointer, g.TMSI)
}

func (s STMSI) String() string {
	return fmt.Sprintf("%03x-%02x-%x", s.AMFSetID, s.AMFPointer, s.TMSI)
}
