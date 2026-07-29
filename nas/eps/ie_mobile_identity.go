// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"fmt"
)

// MobileIdentityType is the type-of-identity field (bits 1-3 of octet 1) of the
// mobile identity IE (TS 24.301 §9.9.2.3 → TS 24.008 §10.5.1.4). Its values
// differ from EPSMobileIdentityType's: IMEI is 010 here and 011 there.
type MobileIdentityType uint8

// Type-of-identity values (TS 24.008 §10.5.1.4, table 10.5.4).
const (
	MobileIdentityNone   MobileIdentityType = 0
	MobileIdentityIMSI   MobileIdentityType = 1
	MobileIdentityIMEI   MobileIdentityType = 2
	MobileIdentityIMEISV MobileIdentityType = 3
	MobileIdentityTMSI   MobileIdentityType = 4
)

var mobileIdentityTypeNames = map[MobileIdentityType]string{
	MobileIdentityNone:   "no identity",
	MobileIdentityIMSI:   "IMSI",
	MobileIdentityIMEI:   "IMEI",
	MobileIdentityIMEISV: "IMEISV",
	MobileIdentityTMSI:   "TMSI",
}

func (t MobileIdentityType) String() string {
	if name, ok := mobileIdentityTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown identity type (%d)", uint8(t))
}

// TypeOfIdentity returns the type of identity carried in the first octet of a
// mobile identity IE value (TS 24.008 §10.5.1.4).
func TypeOfIdentity(octet uint8) MobileIdentityType {
	return MobileIdentityType(octet & 0x07)
}

// MobileIdentity is a mobile identity IE value (TS 24.301 §9.9.2.3 →
// TS 24.008 §10.5.1.4), the IE an EPS IDENTITY RESPONSE and the IMEISV of a
// SECURITY MODE COMPLETE carry. Type selects which of the variant fields carries
// the identity; the others are nil. A type this package does not model keeps its
// value in Raw, so the element re-encodes unchanged.
//
// EPSMobileIdentity is the other, differently coded identity IE of the same
// generation (TS 24.301 §9.9.3.12).
type MobileIdentity struct {
	IMSI   *IMSI
	IMEI   *IMEI
	IMEISV *IMEISV
	TMSI   *[4]byte

	// Raw is the value part verbatim, including the first octet, for a type with
	// no variant field.
	Raw []byte
}

// Type reports which identity this value carries. It is derived rather than
// stored, so the type can never disagree with the payload.
func (m MobileIdentity) Type() MobileIdentityType {
	switch {
	case m.IMSI != nil:
		return MobileIdentityIMSI
	case m.IMEI != nil:
		return MobileIdentityIMEI
	case m.IMEISV != nil:
		return MobileIdentityIMEISV
	case m.TMSI != nil:
		return MobileIdentityTMSI
	case len(m.Raw) > 0:
		return TypeOfIdentity(m.Raw[0])
	default:
		return MobileIdentityNone
	}
}

// MobileIMSI builds a mobile identity carrying an IMSI.
func MobileIMSI(i IMSI) MobileIdentity { return MobileIdentity{IMSI: &i} }

// MobileIMEI builds a mobile identity carrying an IMEI.
func MobileIMEI(i IMEI) MobileIdentity { return MobileIdentity{IMEI: &i} }

// MobileIMEISV builds a mobile identity carrying an IMEISV.
func MobileIMEISV(i IMEISV) MobileIdentity {
	return MobileIdentity{IMEISV: &i}
}

// MobileTMSI builds a mobile identity carrying a TMSI, P-TMSI or M-TMSI.
func MobileTMSI(t [4]byte) MobileIdentity {
	return MobileIdentity{TMSI: &t}
}

// NoIdentity is the "no identity" mobile identity (TS 24.008 §10.5.1.4). It is
// the EPS counterpart of fgs.NoIdentity.
func NoIdentity() MobileIdentity { return MobileIdentity{} }

func (m MobileIdentity) String() string {
	switch {
	case m.IMSI != nil:
		return m.IMSI.String()
	case m.IMEI != nil:
		return m.IMEI.String()
	case m.IMEISV != nil:
		return m.IMEISV.String()
	case m.TMSI != nil:
		return fmt.Sprintf("tmsi-%x", *m.TMSI)
	default:
		return m.Type().String()
	}
}

// maxMobileIdentityLen is the element's longest value: TS 24.008 §10.5.1.4 caps
// the whole element at 11 octets, two of which are its IEI and length.
const maxMobileIdentityLen = 9

// ParseMobileIdentity decodes a mobile identity IE value.
func ParseMobileIdentity(b []byte) (MobileIdentity, error) {
	if len(b) == 0 {
		return MobileIdentity{}, fmt.Errorf("nas/eps: empty mobile identity")
	}

	if len(b) > maxMobileIdentityLen {
		return MobileIdentity{}, fmt.Errorf(
			"nas/eps: mobile identity is %d octets, want at most %d", len(b), maxMobileIdentityLen)
	}

	typ := TypeOfIdentity(b[0])

	switch typ {
	case MobileIdentityIMSI, MobileIdentityIMEI, MobileIdentityIMEISV:
		digits, err := parsePackedDigits(b, typ)
		if err != nil {
			return MobileIdentity{}, err
		}

		switch typ {
		case MobileIdentityIMSI:
			return MobileIMSI(IMSI(digits)), nil
		case MobileIdentityIMEI:
			return MobileIMEI(IMEI(digits)), nil
		default:
			return MobileIMEISV(IMEISV(digits)), nil
		}
	case MobileIdentityTMSI:
		// TS 24.008 §10.5.1.4: the TMSI/P-TMSI/M-TMSI is four octets, after the
		// octet carrying the type, so the value is exactly five.
		if len(b) != 5 {
			return MobileIdentity{}, fmt.Errorf("nas/eps: TMSI mobile identity is %d octets, want 5", len(b))
		}

		return MobileTMSI([4]byte(b[1:5])), nil
	default:
		return MobileIdentity{Raw: bytes.Clone(b)}, nil
	}
}

// AppendBinary encodes the mobile identity IE value onto b.
func (m MobileIdentity) AppendBinary(b []byte) ([]byte, error) {
	out, err := m.appendValue(b)
	if err != nil {
		return b, err
	}

	if len(out)-len(b) > maxMobileIdentityLen {
		return b, fmt.Errorf(
			"nas/eps: mobile identity is %d octets, want at most %d", len(out)-len(b), maxMobileIdentityLen)
	}

	return out, nil
}

func (m MobileIdentity) appendValue(b []byte) ([]byte, error) {
	switch {
	case m.IMSI != nil:
		return appendPackedDigits(b, uint8(MobileIdentityIMSI), MobileIdentityIMSI, string(*m.IMSI))
	case m.IMEI != nil:
		return appendPackedDigits(b, uint8(MobileIdentityIMEI), MobileIdentityIMEI, string(*m.IMEI))
	case m.IMEISV != nil:
		return appendPackedDigits(b, uint8(MobileIdentityIMEISV), MobileIdentityIMEISV, string(*m.IMEISV))
	case m.TMSI != nil:
		// The digit half-octet of octet 1 is a filler for a TMSI
		// (TS 24.008 §10.5.1.4, figure 10.5.4).
		return append(append(b, 0xF0|uint8(MobileIdentityTMSI)), m.TMSI[:]...), nil
	case m.Raw != nil:
		return append(b, m.Raw...), nil
	default:
		// The identity digits are all zero, and the EMM identification procedure
		// fixes the contents length at 3 (TS 24.008 §10.5.1.4, table 10.5.4).
		return append(b, uint8(MobileIdentityNone), 0x00, 0x00), nil
	}
}

// MarshalBinary encodes the mobile identity IE value.
func (m MobileIdentity) MarshalBinary() ([]byte, error) { return m.AppendBinary(nil) }

// IMEISV is an international mobile equipment identity and software version: an
// IMEI's first 14 digits followed by a 2-digit software version, so 16 digits in
// all (TS 23.003 §6.2.2).
type IMEISV string

func (i IMEISV) String() string { return "imeisv-" + string(i) }

// Valid reports whether the identity has the 16 digits TS 23.003 §6.2.2 defines.
func (i IMEISV) Valid() bool {
	if len(i) != 16 {
		return false
	}

	for j := range len(i) {
		if i[j] < '0' || i[j] > '9' {
			return false
		}
	}

	return true
}

// AppendBinary encodes the IMEISV as a mobile identity value onto b.
func (i IMEISV) AppendBinary(b []byte) ([]byte, error) {
	return appendPackedDigits(b, uint8(MobileIdentityIMEISV), MobileIdentityIMEISV, string(i))
}

// MarshalBinary encodes the IMEISV as a mobile identity value.
func (i IMEISV) MarshalBinary() ([]byte, error) { return i.AppendBinary(nil) }
