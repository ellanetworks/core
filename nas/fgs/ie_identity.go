// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/nas"
)

// MobileIdentityType is the type-of-identity field (bits 1-3 of octet 1) of a 5GS
// mobile identity (TS 24.501 §9.11.3.4).
type MobileIdentityType uint8

// Type-of-identity values (TS 24.501 §9.11.3.4, table 9.11.3.4.1).
const (
	IdentityNoIdentity MobileIdentityType = 0
	IdentitySUCI       MobileIdentityType = 1
	IdentityGUTI       MobileIdentityType = 2
	IdentityIMEI       MobileIdentityType = 3
	IdentitySTMSI      MobileIdentityType = 4
	IdentityIMEISV     MobileIdentityType = 5
	IdentityMACAddress MobileIdentityType = 6
	IdentityEUI64      MobileIdentityType = 7
)

var mobileIdentityTypeNames = map[MobileIdentityType]string{
	IdentityNoIdentity: "no identity",
	IdentitySUCI:       "SUCI",
	IdentityGUTI:       "5G-GUTI",
	IdentityIMEI:       "IMEI",
	IdentitySTMSI:      "5G-S-TMSI",
	IdentityIMEISV:     "IMEISV",
	IdentityMACAddress: "MAC address",
	IdentityEUI64:      "EUI-64",
}

func (t MobileIdentityType) String() string {
	if name, ok := mobileIdentityTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown identity type (%d)", uint8(t))
}

// TypeOfIdentity returns the type of identity carried in the first octet of a 5GS
// mobile identity IE value (TS 24.501 §9.11.3.4).
func TypeOfIdentity(octet uint8) MobileIdentityType {
	return MobileIdentityType(octet & 0x07)
}

// MobileIdentity is a 5GS mobile identity IE value (TS 24.501 §9.11.3.4). Type
// selects which of the variant fields carries the identity; the others are nil.
// A type this package does not model keeps its value in Raw, so the element
// re-encodes unchanged.
//
// It is the 5GS counterpart of eps.MobileIdentity, which carries the types
// TS 24.301 defines instead.
type MobileIdentity struct {
	SUCI  *SUCI
	GUTI  *GUTI
	STMSI *STMSI
	PEI   *PEI

	// Raw is the value part verbatim, including the first octet, for a type with
	// no variant field: no identity, MAC address and EUI-64.
	Raw []byte
}

// Type reports which identity this value carries. It is derived rather than
// stored, so the type can never disagree with the payload.
func (m MobileIdentity) Type() MobileIdentityType {
	switch {
	case m.SUCI != nil:
		return IdentitySUCI
	case m.GUTI != nil:
		return IdentityGUTI
	case m.STMSI != nil:
		return IdentitySTMSI
	case m.PEI != nil:
		return m.PEI.Type
	case len(m.Raw) > 0:
		return TypeOfIdentity(m.Raw[0])
	default:
		return IdentityNoIdentity
	}
}

// SUCIIdentity builds a 5GS mobile identity carrying a SUCI.
func SUCIIdentity(s SUCI) MobileIdentity { return MobileIdentity{SUCI: &s} }

// GUTIIdentity builds a 5GS mobile identity carrying a 5G-GUTI.
func GUTIIdentity(g GUTI) MobileIdentity { return MobileIdentity{GUTI: &g} }

// STMSIIdentity builds a 5GS mobile identity carrying a 5G-S-TMSI.
func STMSIIdentity(s STMSI) MobileIdentity { return MobileIdentity{STMSI: &s} }

// PEIIdentity builds a 5GS mobile identity carrying a PEI (an IMEI or IMEISV).
func PEIIdentity(p PEI) MobileIdentity { return MobileIdentity{PEI: &p} }

// NoIdentity is the "no identity" mobile identity a UE sends when it has none to
// offer (TS 24.501 §9.11.3.4).
func NoIdentity() MobileIdentity {
	return MobileIdentity{Raw: []byte{uint8(IdentityNoIdentity)}}
}

func (m MobileIdentity) String() string {
	switch {
	case m.SUCI != nil:
		return m.SUCI.String()
	case m.GUTI != nil:
		return m.GUTI.String()
	case m.STMSI != nil:
		return m.STMSI.String()
	case m.PEI != nil:
		return m.PEI.String()
	default:
		return m.Type().String()
	}
}

// ParseMobileIdentity decodes a 5GS mobile identity IE value.
func ParseMobileIdentity(b []byte) (MobileIdentity, error) {
	if len(b) == 0 {
		return MobileIdentity{}, fmt.Errorf("nas/fgs: empty 5GS mobile identity")
	}

	typ := TypeOfIdentity(b[0])

	switch typ {
	case IdentitySUCI:
		s, err := ParseSUCI(b)
		if err != nil {
			return MobileIdentity{}, err
		}

		return SUCIIdentity(s), nil
	case IdentityGUTI:
		g, err := ParseGUTI(b)
		if err != nil {
			return MobileIdentity{}, err
		}

		return GUTIIdentity(g), nil
	case IdentitySTMSI:
		s, err := ParseSTMSI(b)
		if err != nil {
			return MobileIdentity{}, err
		}

		return STMSIIdentity(s), nil
	case IdentityIMEI, IdentityIMEISV:
		p, err := ParsePEI(b)
		if err != nil {
			return MobileIdentity{}, err
		}

		return PEIIdentity(p), nil
	default:
		return MobileIdentity{Raw: bytes.Clone(b)}, nil
	}
}

// AppendBinary encodes the 5GS mobile identity IE value onto b.
func (m MobileIdentity) AppendBinary(b []byte) ([]byte, error) {
	switch {
	case m.SUCI != nil:
		return m.SUCI.AppendBinary(b)
	case m.GUTI != nil:
		return m.GUTI.AppendBinary(b)
	case m.STMSI != nil:
		return m.STMSI.AppendBinary(b)
	case m.PEI != nil:
		return m.PEI.AppendBinary(b)
	case m.Raw != nil:
		return append(b, m.Raw...), nil
	default:
		// No variant set is "no identity" (TS 24.501 table 9.11.3.4.1), which is
		// what the zero value means and what Type reports for it.
		return append(b, uint8(IdentityNoIdentity)), nil
	}
}

// MarshalBinary encodes the 5GS mobile identity IE value.
func (m MobileIdentity) MarshalBinary() ([]byte, error) { return m.AppendBinary(nil) }

// SUPIFormat is the SUPI format field of a SUCI (TS 24.501 §9.11.3.4, octet 1
// bits 5-8).
type SUPIFormat uint8

// SUPI format values (TS 24.501 §9.11.3.4, table 9.11.3.4.1).
const (
	SUPIFormatIMSI SUPIFormat = 0
	// SUPIFormatNetworkSpecific, SUPIFormatGCI and SUPIFormatGLI all carry the
	// SUPI as a network access identifier, so all three take the NAI layout of
	// figure 9.11.3.4.4; only the IMSI format uses the PLMN layout. Every other
	// value is interpreted as IMSI (TS 24.501 table 9.11.3.4.1).
	SUPIFormatNetworkSpecific SUPIFormat = 1
	SUPIFormatGCI             SUPIFormat = 2
	SUPIFormatGLI             SUPIFormat = 3
)

// IsNAI reports whether this format carries the SUPI as a network access
// identifier rather than as a PLMN, routing indicator and scheme output.
func (f SUPIFormat) IsNAI() bool {
	return f == SUPIFormatNetworkSpecific || f == SUPIFormatGCI || f == SUPIFormatGLI
}

func (f SUPIFormat) String() string {
	switch f {
	case SUPIFormatIMSI:
		return "IMSI"
	case SUPIFormatNetworkSpecific:
		return "network specific identifier"
	case SUPIFormatGCI:
		return "GCI"
	case SUPIFormatGLI:
		return "GLI"
	default:
		return fmt.Sprintf("unknown SUPI format (%d)", uint8(f))
	}
}

// ProtectionScheme is the SUCI protection scheme identifier (TS 24.501
// §9.11.3.4, bits 1-4 of the scheme octet), which names the elliptic-curve
// profile the UE used to conceal the SUPI (TS 33.501 Annex C).
type ProtectionScheme uint8

// SUCI protection schemes (TS 33.501 Annex C.1). Values 3 to 11 are reserved;
// 12 to 15 are for schemes the home network defines.
const (
	// ProtectionSchemeNull leaves the scheme output as the cleartext MSIN.
	ProtectionSchemeNull     ProtectionScheme = 0
	ProtectionSchemeProfileA ProtectionScheme = 1
	ProtectionSchemeProfileB ProtectionScheme = 2
)

func (p ProtectionScheme) String() string {
	switch {
	case p == ProtectionSchemeNull:
		return "null scheme"
	case p == ProtectionSchemeProfileA:
		return "ECIES profile A"
	case p == ProtectionSchemeProfileB:
		return "ECIES profile B"
	case p >= 12 && p <= 15:
		return fmt.Sprintf("home network scheme (%d)", uint8(p))
	default:
		return fmt.Sprintf("reserved protection scheme (%d)", uint8(p))
	}
}

// SUCI is a subscription concealed identifier (TS 24.501 §9.11.3.4,
// TS 23.003 §2.2B). Under the IMSI SUPI format it carries the PLMN in the clear
// and conceals only the MSIN; under the NAI format the whole identifier is
// opaque to the serving network.
type SUCI struct {
	Format SUPIFormat

	// IMSI SUPI format.
	PLMN nas.PLMN
	// RoutingIndicator is 1 to 4 decimal digits. A UE with none configured sends
	// the single digit "0" (TS 24.501 table 9.11.3.4.1, TS 23.003 §2.2B), which is
	// what decoding yields for an element carrying no digits at all.
	RoutingIndicator string
	ProtectionScheme ProtectionScheme
	HomeNetworkPKID  uint8
	// SchemeOutput is the concealed part verbatim. Under ProtectionSchemeNull it
	// is the MSIN in swapped-nibble BCD, which MSIN decodes.
	SchemeOutput []byte

	// NAI SUPI format.
	NAI []byte
}

// MSIN returns the subscriber identification number and whether the scheme
// output revealed it, which only the null protection scheme does.
func (s SUCI) MSIN() (string, bool) {
	if s.Format != SUPIFormatIMSI || s.ProtectionScheme != ProtectionSchemeNull {
		return "", false
	}

	return nas.DecodeTBCD(s.SchemeOutput), true
}

// SUPI returns the subscription permanent identifier as "imsi-<digits>" and
// whether it could be recovered, which needs the IMSI SUPI format and the null
// protection scheme (TS 23.003 §2.2A).
func (s SUCI) SUPI() (string, bool) {
	msin, ok := s.MSIN()
	if !ok {
		return "", false
	}

	return "imsi-" + s.PLMN.MCC + s.PLMN.MNC + msin, true
}

// String renders the SUCI in the dashed form the 5G core uses on its service
// based interfaces (TS 29.571): "suci-0-<mcc>-<mnc>-<routing>-<scheme>-<hnpki>-
// <output>" for the IMSI SUPI format, and "nai-<format>-<nai>" for the three
// formats that carry a network access identifier, whose field is a UTF-8 string
// (TS 24.501 table 9.11.3.4.1).
func (s SUCI) String() string {
	if s.Format.IsNAI() {
		return fmt.Sprintf("nai-%d-%s", uint8(s.Format), s.NAI)
	}

	output := fmt.Sprintf("%x", s.SchemeOutput)
	if msin, ok := s.MSIN(); ok {
		output = msin
	}

	return strings.Join([]string{
		"suci", "0", s.PLMN.MCC, s.PLMN.MNC, s.RoutingIndicator,
		fmt.Sprintf("%x", uint8(s.ProtectionScheme)), fmt.Sprintf("%d", s.HomeNetworkPKID), output,
	}, "-")
}

// ParseSUCI decodes a SUCI 5GS mobile identity value.
func ParseSUCI(b []byte) (SUCI, error) {
	r := nas.NewReader(b)

	head, err := r.U8()
	if err != nil {
		return SUCI{}, err
	}

	if typ := TypeOfIdentity(head); typ != IdentitySUCI {
		return SUCI{}, fmt.Errorf("nas/fgs: mobile identity type %s is not a SUCI", typ)
	}

	out := SUCI{Format: SUPIFormat(head >> 4 & 0x07)}

	if out.Format.IsNAI() {
		if out.NAI, err = r.Bytes(r.Remaining()); err != nil {
			return SUCI{}, err
		}

		return out, nil
	}

	plmn, err := r.Bytes(3)
	if err != nil {
		return SUCI{}, err
	}

	if out.PLMN, err = nas.ParsePLMN([3]byte{plmn[0], plmn[1], plmn[2]}); err != nil {
		return SUCI{}, err
	}

	routing, err := r.Bytes(2)
	if err != nil {
		return SUCI{}, err
	}

	out.RoutingIndicator = nas.DecodeTBCD(routing)

	// An element whose routing indicator is all fillers carries no digits, which
	// means the same as the "0" a conformant sender writes; naming it the same way
	// keeps the two encodings from denoting different values.
	if out.RoutingIndicator == "" {
		out.RoutingIndicator = "0"
	}

	scheme, err := r.U8()
	if err != nil {
		return SUCI{}, err
	}

	out.ProtectionScheme = ProtectionScheme(scheme & 0x0F)

	if out.HomeNetworkPKID, err = r.U8(); err != nil {
		return SUCI{}, err
	}

	if out.SchemeOutput, err = r.Bytes(r.Remaining()); err != nil {
		return SUCI{}, err
	}

	return out, nil
}

// AppendBinary encodes the SUCI as a 5GS mobile identity value onto b.
func (s SUCI) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(uint8(s.Format)&0x07<<4 | uint8(IdentitySUCI))

	if s.Format.IsNAI() {
		w.Raw(s.NAI)

		return w.Result(b)
	}

	plmn, err := s.PLMN.Octets()
	if err != nil {
		return b, err
	}

	w.Raw(plmn[:])

	// The routing indicator occupies two octets whatever its digit count, padded
	// with 0xF fillers (TS 24.501 §9.11.3.4). A UE with none configured sends the
	// single digit 0 rather than an empty field, so the octets read F0 FF.
	digits := s.RoutingIndicator
	if digits == "" {
		digits = "0"
	}

	routing, err := nas.EncodeTBCD(digits)
	if err != nil {
		return b, err
	}

	if len(routing) > 2 {
		return b, fmt.Errorf("nas/fgs: routing indicator %q exceeds 4 digits", s.RoutingIndicator)
	}

	for len(routing) < 2 {
		routing = append(routing, 0xFF)
	}

	w.Raw(routing)
	w.U8(uint8(s.ProtectionScheme) & 0x0F)
	w.U8(s.HomeNetworkPKID)
	w.Raw(s.SchemeOutput)

	return w.Result(b)
}

// MarshalBinary encodes the SUCI as a 5GS mobile identity value.
func (s SUCI) MarshalBinary() ([]byte, error) { return s.AppendBinary(nil) }

// PEI is a permanent equipment identifier carried as a 5GS mobile identity: an
// IMEI (15 digits) or an IMEISV (16 digits), TS 24.501 §9.11.3.4 and
// TS 23.003 §6.
type PEI struct {
	Type   MobileIdentityType // IdentityIMEI or IdentityIMEISV
	Digits string
}

func (p PEI) String() string {
	if p.Type == IdentityIMEI {
		return "imei-" + p.Digits
	}

	return "imeisv-" + p.Digits
}

// Valid reports whether the identifier has the length its type requires and, for
// an IMEI, a correct Luhn check digit (TS 23.003 §6.2.1).
func (p PEI) Valid() bool {
	if p.Type == IdentityIMEI {
		return imeiValid(p.Digits)
	}

	return p.Type == IdentityIMEISV && len(p.Digits) == 16
}

// ParsePEI decodes an IMEI or IMEISV 5GS mobile identity value. It does not
// enforce the length or check digit — use Valid for that — so that an identifier
// the network will reject still round-trips.
func ParsePEI(b []byte) (PEI, error) {
	if len(b) == 0 {
		return PEI{}, fmt.Errorf("nas/fgs: empty PEI")
	}

	typ := TypeOfIdentity(b[0])
	if typ != IdentityIMEI && typ != IdentityIMEISV {
		return PEI{}, fmt.Errorf("nas/fgs: mobile identity type %s is not a PEI", typ)
	}

	// The first digit shares octet 1 with the odd/even indication and the type;
	// the rest follow two per octet, low nibble first, ending at the filler that
	// pads an even digit count (TS 23.003 §6.2).
	if b[0]>>4 > 9 {
		return PEI{}, fmt.Errorf("nas/fgs: PEI first digit is not decimal")
	}

	digits := string('0'+b[0]>>4) + nas.DecodeTBCD(b[1:])

	// The filler and the odd/even indication both encode the digit count; a value
	// where they disagree is malformed rather than ambiguous.
	if odd := uint8(len(digits) & 1); odd != b[0]>>3&0x01 {
		return PEI{}, fmt.Errorf("nas/fgs: PEI has %d digits but its odd/even indication says otherwise", len(digits))
	}

	return PEI{Type: typ, Digits: digits}, nil
}

// AppendBinary encodes the PEI as a 5GS mobile identity value onto b.
func (p PEI) AppendBinary(b []byte) ([]byte, error) {
	if p.Type != IdentityIMEI && p.Type != IdentityIMEISV {
		return b, fmt.Errorf("nas/fgs: mobile identity type %s is not a PEI", p.Type)
	}

	if len(p.Digits) == 0 {
		return b, fmt.Errorf("nas/fgs: PEI has no digits")
	}

	first, rest := p.Digits[0], p.Digits[1:]
	if first < '0' || first > '9' {
		return b, fmt.Errorf("nas/fgs: PEI digit %q is not decimal", first)
	}

	packed, err := nas.EncodeTBCD(rest)
	if err != nil {
		return b, err
	}

	odd := uint8(len(p.Digits) & 1)

	return append(append(b, (first-'0')<<4|odd<<3|uint8(p.Type)), packed...), nil
}

// MarshalBinary encodes the PEI as a 5GS mobile identity value.
func (p PEI) MarshalBinary() ([]byte, error) { return p.AppendBinary(nil) }

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
