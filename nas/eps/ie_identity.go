// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// EPSMobileIdentityType is the type-of-identity field (bits 1-3 of octet 1) of an
// EPS mobile identity (TS 24.301 §9.9.3.12). It is not the coding of the mobile
// identity IE of TS 24.008, which MobileIdentityType carries.
type EPSMobileIdentityType uint8

// Type-of-identity values (TS 24.301 §9.9.3.12).
const (
	IdentityIMSI EPSMobileIdentityType = 1
	IdentityIMEI EPSMobileIdentityType = 3
	IdentityGUTI EPSMobileIdentityType = 6
)

var epsMobileIdentityTypeNames = map[EPSMobileIdentityType]string{
	IdentityIMSI: "IMSI",
	IdentityIMEI: "IMEI",
	IdentityGUTI: "GUTI",
}

func (t EPSMobileIdentityType) String() string {
	if name, ok := epsMobileIdentityTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown identity type (%d)", uint8(t))
}

// EPSTypeOfIdentity returns the type of identity carried in the first octet of
// an EPS mobile identity IE value (TS 24.301 §9.9.3.12).
func EPSTypeOfIdentity(octet uint8) EPSMobileIdentityType {
	return EPSMobileIdentityType(octet & 0x07)
}

// EPSMobileIdentity is an EPS mobile identity IE value (TS 24.301 §9.9.3.12).
// Type selects which of the variant fields carries the identity; the others are
// nil. A type this package does not model keeps its value in Raw, so the element
// re-encodes unchanged.
//
// It is the EPS counterpart of fgs.MobileIdentity. 5GS folded every identity
// into one IE; EPS kept the mobile identity IE of TS 24.008 alongside this one,
// with a different type coding, which MobileIdentity carries.
type EPSMobileIdentity struct {
	GUTI *GUTI
	IMSI *IMSI
	IMEI *IMEI

	// Raw is the value part verbatim, including the first octet, for a type with
	// no variant field.
	Raw []byte
}

// Type reports which identity this value carries. It is derived rather than
// stored, so the type can never disagree with the payload.
func (m EPSMobileIdentity) Type() EPSMobileIdentityType {
	switch {
	case m.GUTI != nil:
		return IdentityGUTI
	case m.IMSI != nil:
		return IdentityIMSI
	case m.IMEI != nil:
		return IdentityIMEI
	case len(m.Raw) > 0:
		return EPSTypeOfIdentity(m.Raw[0])
	default:
		return 0
	}
}

// GUTIIdentity builds an EPS mobile identity carrying a GUTI.
func GUTIIdentity(g GUTI) EPSMobileIdentity { return EPSMobileIdentity{GUTI: &g} }

// IMSIIdentity builds an EPS mobile identity carrying an IMSI.
func IMSIIdentity(i IMSI) EPSMobileIdentity { return EPSMobileIdentity{IMSI: &i} }

// IMEIIdentity builds an EPS mobile identity carrying an IMEI.
func IMEIIdentity(i IMEI) EPSMobileIdentity { return EPSMobileIdentity{IMEI: &i} }

func (m EPSMobileIdentity) String() string {
	switch {
	case m.GUTI != nil:
		return m.GUTI.String()
	case m.IMSI != nil:
		return m.IMSI.String()
	case m.IMEI != nil:
		return m.IMEI.String()
	default:
		return m.Type().String()
	}
}

// maxEPSMobileIdentityLen is the element's longest value: TS 24.301 §9.9.3.12
// caps the whole element at 13 octets, two of which are its IEI and length.
const maxEPSMobileIdentityLen = 11

// ParseEPSMobileIdentity decodes an EPS mobile identity IE value.
func ParseEPSMobileIdentity(b []byte) (EPSMobileIdentity, error) {
	if len(b) == 0 {
		return EPSMobileIdentity{}, fmt.Errorf("nas/eps: empty EPS mobile identity")
	}

	if len(b) > maxEPSMobileIdentityLen {
		return EPSMobileIdentity{}, fmt.Errorf(
			"nas/eps: EPS mobile identity is %d octets, want at most %d", len(b), maxEPSMobileIdentityLen)
	}

	typ := EPSTypeOfIdentity(b[0])

	switch typ {
	case IdentityGUTI:
		g, err := ParseGUTI(b)
		if err != nil {
			return EPSMobileIdentity{}, err
		}

		return GUTIIdentity(g), nil
	case IdentityIMSI:
		digits, err := parsePackedDigits(b, IdentityIMSI)
		if err != nil {
			return EPSMobileIdentity{}, err
		}

		return IMSIIdentity(IMSI(digits)), nil
	case IdentityIMEI:
		digits, err := parsePackedDigits(b, IdentityIMEI)
		if err != nil {
			return EPSMobileIdentity{}, err
		}

		return IMEIIdentity(IMEI(digits)), nil
	default:
		return EPSMobileIdentity{Raw: bytes.Clone(b)}, nil
	}
}

// AppendBinary encodes the EPS mobile identity IE value onto b.
func (m EPSMobileIdentity) AppendBinary(b []byte) ([]byte, error) {
	out, err := m.appendValue(b)
	if err != nil {
		return b, err
	}

	if len(out)-len(b) > maxEPSMobileIdentityLen {
		return b, fmt.Errorf(
			"nas/eps: EPS mobile identity is %d octets, want at most %d", len(out)-len(b), maxEPSMobileIdentityLen)
	}

	return out, nil
}

func (m EPSMobileIdentity) appendValue(b []byte) ([]byte, error) {
	switch {
	case m.GUTI != nil:
		return m.GUTI.AppendBinary(b)
	case m.IMSI != nil:
		return m.IMSI.AppendBinary(b)
	case m.IMEI != nil:
		return m.IMEI.AppendBinary(b)
	case m.Raw != nil:
		return append(b, m.Raw...), nil
	default:
		return b, fmt.Errorf("nas/eps: %s mobile identity carries no value", m.Type())
	}
}

// MarshalBinary encodes the EPS mobile identity IE value.
func (m EPSMobileIdentity) MarshalBinary() ([]byte, error) { return m.AppendBinary(nil) }

// GUTI is a globally unique temporary identity (TS 23.003 §2.8): the PLMN and
// MME identity that allocated it, and the M-TMSI it allocated. It is the EPS
// counterpart of fgs.GUTI.
type GUTI struct {
	PLMN       nas.PLMN
	MMEGroupID uint16
	MMECode    uint8
	TMSI       [4]byte // M-TMSI
}

func (g GUTI) String() string {
	return fmt.Sprintf("%s-%04x-%02x-%x", g.PLMN, g.MMEGroupID, g.MMECode, g.TMSI)
}

// ParseGUTI decodes a GUTI EPS mobile identity value (11 octets: type, PLMN,
// MME group ID, MME code, M-TMSI).
func ParseGUTI(b []byte) (GUTI, error) {
	if len(b) != 11 {
		return GUTI{}, fmt.Errorf("nas/eps: GUTI is %d octets, want 11", len(b))
	}

	if typ := EPSTypeOfIdentity(b[0]); typ != IdentityGUTI {
		return GUTI{}, fmt.Errorf("nas/eps: mobile identity type %s is not a GUTI", typ)
	}

	plmn, err := nas.ParsePLMN([3]byte{b[1], b[2], b[3]})
	if err != nil {
		return GUTI{}, err
	}

	return GUTI{
		PLMN:       plmn,
		MMEGroupID: binary.BigEndian.Uint16(b[4:6]),
		MMECode:    b[6],
		TMSI:       [4]byte(b[7:11]),
	}, nil
}

// AppendBinary encodes the GUTI as its EPS mobile identity value onto b.
func (g GUTI) AppendBinary(b []byte) ([]byte, error) {
	plmn, err := g.PLMN.Octets()
	if err != nil {
		return b, err
	}

	b = append(b, 0xF0|uint8(IdentityGUTI)) // spare 1111, type-of-identity 110
	b = append(b, plmn[:]...)
	b = binary.BigEndian.AppendUint16(b, g.MMEGroupID)
	b = append(b, g.MMECode)
	b = append(b, g.TMSI[:]...)

	return b, nil
}

// MarshalBinary encodes the GUTI as its EPS mobile identity value.
func (g GUTI) MarshalBinary() ([]byte, error) { return g.AppendBinary(nil) }

// IMSI is an international mobile subscriber identity carried as an EPS mobile
// identity: up to 15 decimal digits (TS 23.003 §2.2).
type IMSI string

func (i IMSI) String() string { return "imsi-" + string(i) }

// AppendBinary encodes the IMSI as an EPS mobile identity value onto b.
func (i IMSI) AppendBinary(b []byte) ([]byte, error) {
	return appendPackedDigits(b, uint8(IdentityIMSI), IdentityIMSI, string(i))
}

// MarshalBinary encodes the IMSI as an EPS mobile identity value.
func (i IMSI) MarshalBinary() ([]byte, error) { return i.AppendBinary(nil) }

// IMEI is an international mobile equipment identity carried as an EPS mobile
// identity: 15 decimal digits ending in a Luhn check digit (TS 23.003 §6.2).
type IMEI string

func (i IMEI) String() string { return "imei-" + string(i) }

// Valid reports whether the identity is 15 digits with a correct check digit.
func (i IMEI) Valid() bool { return imeiValid(string(i)) }

// AppendBinary encodes the IMEI as an EPS mobile identity value onto b.
func (i IMEI) AppendBinary(b []byte) ([]byte, error) {
	return appendPackedDigits(b, uint8(IdentityIMEI), IdentityIMEI, string(i))
}

// MarshalBinary encodes the IMEI as an EPS mobile identity value.
func (i IMEI) MarshalBinary() ([]byte, error) { return i.AppendBinary(nil) }

// parsePackedDigits decodes the digit-packed identity forms shared by the two
// identity IEs: the first digit shares octet 1 with the odd/even indication and
// the type, the rest follow two per octet, low nibble first, ending at the
// filler that pads an even digit count (TS 24.301 §9.9.3.12, TS 24.008
// §10.5.1.4).
func parsePackedDigits(b []byte, what fmt.Stringer) (string, error) {
	if b[0]>>4 > 9 {
		return "", fmt.Errorf("nas/eps: %s first digit is not decimal", what)
	}

	rest, err := nas.DecodeTBCD(b[1:])
	if err != nil {
		return "", fmt.Errorf("nas/eps: %s: %w", what, err)
	}

	digits := string('0'+b[0]>>4) + rest

	// The filler and the odd/even indication both encode the digit count; a value
	// where they disagree is malformed rather than ambiguous.
	if odd := uint8(len(digits) & 1); odd != b[0]>>3&0x01 {
		return "", fmt.Errorf("nas/eps: %s has %d digits but its odd/even indication says otherwise", what, len(digits))
	}

	return digits, nil
}

func appendPackedDigits(b []byte, typ uint8, what fmt.Stringer, digits string) ([]byte, error) {
	if len(digits) == 0 {
		return nil, fmt.Errorf("nas/eps: %s has no digits", what)
	}

	first, rest := digits[0], digits[1:]
	if first < '0' || first > '9' {
		return nil, fmt.Errorf("nas/eps: %s digit %q is not decimal", what, first)
	}

	packed, err := nas.EncodeTBCD(rest)
	if err != nil {
		return nil, err
	}

	odd := uint8(len(digits) & 1)

	return append(append(b, (first-'0')<<4|odd<<3|typ), packed...), nil
}

// luhnValid reports whether digits pass the Luhn checksum. A non-decimal
// character fails it.
func luhnValid(digits string) bool {
	sum := 0

	for i := len(digits) - 1; i >= 0; i-- {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}

		d := int(digits[i] - '0')

		if (len(digits)-i)%2 == 0 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}

		sum += d
	}

	return sum%10 == 0
}

// imeiValid reports whether digits is a well-formed 15-digit IMEI.
//
// The 15th digit is the Check Digit only when the IMEI is written down; over the
// air it is the Spare Digit, which TS 23.003 §6.2.1 requires the MS to set to
// zero and which the Luhn formula of Annex B is not applied to. Accepting either
// keeps a conformant handset's identity valid without giving up the check on an
// IMEI that does carry one.
func imeiValid(digits string) bool {
	if len(digits) != 15 {
		return false
	}

	for i := range digits {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}

	return digits[14] == '0' || luhnValid(digits)
}
