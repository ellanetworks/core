// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/nas"
)

// ESM information element identifiers (TS 24.301 §9.9.4 message definitions).
const (
	ieiESMInformationTransferFlag   uint8 = 0xD0 // type 1
	ieiProtocolConfigurationOptions uint8 = 0x27
	ieiAccessPointName              uint8 = 0x28
	ieiESMCause                     uint8 = 0x58
	// ieiNewEPSQoS and ieiRequiredTrafficFlowQoS are the same EPS quality of
	// service element under the two names its messages give it.
	ieiNewEPSQoS              uint8 = 0x5B
	ieiRequiredTrafficFlowQoS uint8 = 0x5B
	ieiAPNAMBR                uint8 = 0x5E
)

// These IE codecs produce/consume the *value part* of an information element
// (the bytes inside its LV/LV-E length). The MME composes them into the
// message's LV fields. References are to TS 24.301.

// PDN type values (TS 24.301).
const (
	PDNTypeIPv4     PDNType = 1
	PDNTypeIPv6     PDNType = 2
	PDNTypeIPv4v6   PDNType = 3
	PDNTypeNonIP    PDNType = 5
	PDNTypeEthernet PDNType = 6
)

// PDNAddress is the PDN address: the IP assigned to the UE. IPv4 is
// the 4-octet address; IPv6IID the 8-octet interface identifier.
type PDNAddress struct {
	PDNType PDNType
	IPv4    [4]byte
	IPv6IID [8]byte

	// Spare holds bits 4-8 of the type octet, which TS 24.301 §9.9.4.9 codes as
	// zero. Keeping them lets the element re-encode as it arrived; fgs.PDUAddress
	// keeps its own the same way.
	Spare uint8

	// Rest holds the octets beyond those the PDN type defines: the four spare
	// octets a non-IP or Ethernet connection carries (TS 24.301 §9.9.4.9), and
	// the whole body of a type this release does not assign. Keeping them lets
	// the element re-encode unchanged instead of a reserved type failing the
	// message it arrived in.
	Rest []byte
}

// AppendBinary returns the PDN address value part.
// The encoding is appended to b.
func (a PDNAddress) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(a.Spare<<3 | uint8(a.PDNType&0x07))

	switch a.PDNType {
	case PDNTypeIPv4:
		w.Raw(a.IPv4[:])
	case PDNTypeIPv6:
		w.Raw(a.IPv6IID[:])
	case PDNTypeIPv4v6:
		w.Raw(a.IPv6IID[:])
		w.Raw(a.IPv4[:])
	}

	w.Raw(a.Rest)

	return w.Result(b)
}

// MarshalBinary encodes the PDNAddress information element value.
func (a PDNAddress) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// IPv4Addr returns the IPv4 address the IE carries, or the zero Addr when the
// PDN type carries none.
func (a PDNAddress) IPv4Addr() netip.Addr {
	if a.PDNType != PDNTypeIPv4 && a.PDNType != PDNTypeIPv4v6 {
		return netip.Addr{}
	}

	return netip.AddrFrom4(a.IPv4)
}

// IPv6LinkLocal returns the link-local address formed from the interface
// identifier the IE carries, or the zero Addr when the PDN type carries none.
// The IE never carries a global prefix: the UE obtains one by SLAAC from the
// router advertisement (TS 24.301 §9.9.4.9).
func (a PDNAddress) IPv6LinkLocal() netip.Addr {
	if a.PDNType != PDNTypeIPv6 && a.PDNType != PDNTypeIPv4v6 {
		return netip.Addr{}
	}

	b := [16]byte{0: 0xfe, 1: 0x80}
	copy(b[8:], a.IPv6IID[:])

	return netip.AddrFrom16(b)
}

// ParsePDNAddress decodes a PDN address value part.
func ParsePDNAddress(b []byte) (PDNAddress, error) {
	r := nas.NewReader(b)

	t, err := r.U8()
	if err != nil {
		return PDNAddress{}, err
	}

	a := PDNAddress{PDNType: PDNType(t & 0x07), Spare: t >> 3}

	body, err := r.Bytes(r.Remaining())
	if err != nil {
		return PDNAddress{}, err
	}

	// A non-IP or Ethernet connection carries four spare octets and no address;
	// a type this release does not assign carries nothing this codec can name.
	// Both keep their body in Rest.
	var want int

	switch a.PDNType {
	case PDNTypeIPv4:
		want = 4
	case PDNTypeIPv6:
		want = 8
	case PDNTypeIPv4v6:
		want = 12
	}

	if len(body) < want {
		return PDNAddress{}, fmt.Errorf("nas/eps: PDN address: type %s wants %d octets, got %d", a.PDNType, want, len(body))
	}

	switch a.PDNType {
	case PDNTypeIPv4:
		copy(a.IPv4[:], body)
	case PDNTypeIPv6:
		copy(a.IPv6IID[:], body)
	case PDNTypeIPv4v6:
		copy(a.IPv6IID[:], body[:8])
		copy(a.IPv4[:], body[8:12])
	}

	if len(body) > want {
		a.Rest = body[want:]
	}

	return a, nil
}

// EPSQoS is the EPS quality of service. For a non-GBR default bearer
// only the QCI is present; BitRates holds the optional MBR/GBR octets.
type EPSQoS struct {
	QCI      uint8
	BitRates []byte
}

// AppendBinary returns the EPS QoS value part.
// The encoding is appended to b.
func (q EPSQoS) AppendBinary(b []byte) ([]byte, error) {
	if err := checkEPSQoSLen(1 + len(q.BitRates)); err != nil {
		return b, err
	}

	w := nas.NewWriter(b)

	w.U8(q.QCI)
	w.Raw(q.BitRates)

	return w.Result(b)
}

// checkEPSQoSLen reports whether n is a value length TS 24.301 §9.9.4.3 allows:
// the QCI octet alone, or the QCI octet and one to three groups of four bit-rate
// octets.
func checkEPSQoSLen(n int) error {
	if n < 1 || n > maxEPSQoSLen || (n-1)%epsQoSGroupLen != 0 {
		return fmt.Errorf("nas/eps: EPS QoS is %d octets, want 1, 5, 9 or 13", n)
	}

	return nil
}

// MarshalBinary encodes the EPSQoS information element value.
func (q EPSQoS) MarshalBinary() ([]byte, error) { return q.AppendBinary(nil) }

// EPS QoS value lengths (TS 24.301 §9.9.4.3): the element is 3 to 15 octets, two
// of which are its IEI and length, and its optional octets come in groups of
// four.
const (
	maxEPSQoSLen   = 13
	epsQoSGroupLen = 4
)

// ParseEPSQoS decodes an EPS QoS value part.
func ParseEPSQoS(b []byte) (EPSQoS, error) {
	if err := checkEPSQoSLen(len(b)); err != nil {
		return EPSQoS{}, err
	}

	r := nas.NewReader(b)

	qci, err := r.U8()
	if err != nil {
		return EPSQoS{}, err
	}

	rest, err := remainder(r)
	if err != nil {
		return EPSQoS{}, err
	}

	return EPSQoS{QCI: qci, BitRates: rest}, nil
}

// APN is an Access Point Name in its dotted form, encoded on the wire as RFC 1035
// labels (TS 23.003 §9.1, TS 24.301 §9.9.4.1).
type APN string

// ParseAPN decodes an APN information element value.
func ParseAPN(b []byte) (APN, error) {
	name, err := nas.ParseLabelledName(b)

	return APN(name), err
}

// AppendBinary encodes the APN information element value onto b.
func (a APN) AppendBinary(b []byte) ([]byte, error) { return nas.AppendLabelledName(b, string(a)) }

// MarshalBinary encodes the APN information element value.
func (a APN) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// APNAMBR is the APN aggregate maximum bit rate. DownlinkOctet and
// UplinkOctet use the GPRS bit-rate coding; Extended holds the optional
// higher-rate octets.
type APNAMBR struct {
	DownlinkOctet uint8
	UplinkOctet   uint8
	Extended      []byte
}

// checkAPNAMBRLen reports whether n is a value length TS 24.301 §9.9.4.2 allows:
// the two mandatory octets and up to two pairs of extended octets.
func checkAPNAMBRLen(n int) error {
	if n < 2 || n > maxAPNAMBRLen || n%2 != 0 {
		return fmt.Errorf("nas/eps: APN-AMBR is %d octets, want 2, 4 or 6", n)
	}

	return nil
}

// AppendBinary returns the APN-AMBR value part.
// The encoding is appended to b.
func (a APNAMBR) AppendBinary(b []byte) ([]byte, error) {
	if err := checkAPNAMBRLen(2 + len(a.Extended)); err != nil {
		return b, err
	}

	w := nas.NewWriter(b)

	w.U8(a.DownlinkOctet)
	w.U8(a.UplinkOctet)
	w.Raw(a.Extended)

	return w.Result(b)
}

// MarshalBinary encodes the APNAMBR information element value.
func (a APNAMBR) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// APN-AMBR value lengths (TS 24.301 §9.9.4.2): the element is 4 to 8 octets, two
// of which are its IEI and length, and its optional octets come in pairs.
const maxAPNAMBRLen = 6

// ParseAPNAMBR decodes an APN-AMBR value part.
func ParseAPNAMBR(b []byte) (APNAMBR, error) {
	if err := checkAPNAMBRLen(len(b)); err != nil {
		return APNAMBR{}, err
	}

	r := nas.NewReader(b)

	dl, err := r.U8()
	if err != nil {
		return APNAMBR{}, err
	}

	ul, err := r.U8()
	if err != nil {
		return APNAMBR{}, err
	}

	ext, err := remainder(r)
	if err != nil {
		return APNAMBR{}, err
	}

	return APNAMBR{DownlinkOctet: dl, UplinkOctet: ul, Extended: ext}, nil
}

// apnAMBRExtended2Step is the bit rate one unit of the extended-2 octet adds
// (TS 24.301 §9.9.4.2): that octet is additive on top of octets 3 and 5, not a
// replacement for them.
const apnAMBRExtended2Step uint64 = 256_000_000

// apnAMBRMax is the largest rate the element carries: octet 5 at its maximum
// (256 Mbps) plus 254 extended-2 steps. Above it TS 24.301 §9.9.4.2 directs the
// sender to the extended APN aggregate maximum bit rate IE (§9.9.4.29).
const apnAMBRMax uint64 = 254*apnAMBRExtended2Step + 256_000_000

// APNAMBRFromKbps builds an APN-AMBR carrying the given downlink and uplink
// rates, using the fewest octets that hold them. It is the EPS counterpart of
// fgs.SessionAMBRFromKbps.
//
// The element expresses rates on a coarse ladder, so a rate that falls between
// two steps is rounded down to the nearest one it can express: the value is a
// maximum, and the nearest expressible maximum below it is still a valid one,
// where rounding up would licence more than the caller allowed. Read the value
// back with Kbps to see what was encoded. A rate beyond the element's range
// errors instead, since TS 24.301 §9.9.4.2 carries those in the extended APN
// aggregate maximum bit rate IE (§9.9.4.29) rather than this one.
func APNAMBRFromKbps(downlinkKbps, uplinkKbps uint64) (APNAMBR, error) {
	if downlinkKbps > apnAMBRMax/1000 || uplinkKbps > apnAMBRMax/1000 {
		return APNAMBR{}, fmt.Errorf(
			"nas/eps: APN-AMBR: %d/%d kbit/s exceeds the %d kbit/s this element carries",
			downlinkKbps, uplinkKbps, apnAMBRMax/1000)
	}

	dlBase, dlExt, dlExt2, err := encodeAPNAMBRDirection(downlinkKbps * 1000)
	if err != nil {
		return APNAMBR{}, fmt.Errorf("nas/eps: APN-AMBR downlink: %w", err)
	}

	ulBase, ulExt, ulExt2, err := encodeAPNAMBRDirection(uplinkKbps * 1000)
	if err != nil {
		return APNAMBR{}, fmt.Errorf("nas/eps: APN-AMBR uplink: %w", err)
	}

	a := APNAMBR{DownlinkOctet: dlBase, UplinkOctet: ulBase}

	// The octets come in direction pairs, and a higher pair implies the lower
	// one. A direction that does not need an extension carries 0x00, which means
	// "use the octet below".
	switch {
	case dlExt2 != 0 || ulExt2 != 0:
		a.Extended = []byte{dlExt, ulExt, dlExt2, ulExt2}
	case dlExt != 0 || ulExt != 0:
		a.Extended = []byte{dlExt, ulExt}
	}

	return a, nil
}

// Kbps returns the downlink and uplink rates in kbit/s, and whether both octets
// denote one — TS 24.301 §9.9.4.2 reserves the all-zero octet, which names no
// rate. It is the EPS counterpart of fgs.SessionAMBR.Kbps.
func (a APNAMBR) Kbps() (downlink, uplink uint64, ok bool) {
	var dlExt, ulExt, dlExt2, ulExt2 uint8

	if len(a.Extended) >= 2 {
		dlExt, ulExt = a.Extended[0], a.Extended[1]
	}

	if len(a.Extended) >= 4 {
		dlExt2, ulExt2 = a.Extended[2], a.Extended[3]
	}

	// The base octet is only reserved when no extended octet overrides it.
	if (a.DownlinkOctet == 0 && dlExt == 0) || (a.UplinkOctet == 0 && ulExt == 0) {
		return 0, 0, false
	}

	return decodeAPNAMBRDirection(a.DownlinkOctet, dlExt, dlExt2) / 1000,
		decodeAPNAMBRDirection(a.UplinkOctet, ulExt, ulExt2) / 1000, true
}

// encodeAPNAMBRDirection splits one direction's rate across the base octet
// (3/4), the extended octet (5/6) and the extended-2 octet (7/8).
func encodeAPNAMBRDirection(bps uint64) (base, ext, ext2 uint8, err error) {
	if bps > apnAMBRMax {
		return 0, 0, 0, fmt.Errorf("%d bit/s exceeds the %d bit/s this element carries", bps, apnAMBRMax)
	}

	// The extended-2 octet is additive, so take as many whole steps as fit and
	// leave the remainder to octets 3 and 5. An exact multiple of the step is
	// carried as one step fewer plus a full 256 Mbps remainder, which is what
	// keeps the element's maximum reachable.
	steps := bps / apnAMBRExtended2Step
	rest := bps - steps*apnAMBRExtended2Step

	if steps > 0 && rest == 0 {
		steps--
		rest = apnAMBRExtended2Step
	}

	base, ext = encodeAPNAMBRBase(rest)

	return base, ext, uint8(steps), nil
}

// encodeAPNAMBRBase encodes a rate of at most 256 Mbps into the base octet and,
// where the base octet's granularity cannot hold it, the extended octet. Each
// ladder floors to its own step and is clamped to its own top, so the result
// never reads as a value from the ladder above.
func encodeAPNAMBRBase(bps uint64) (base, ext uint8) {
	kbps := bps / 1000

	switch {
	case kbps == 0:
		return 0xFF, 0 // the base octet's "0 kbps"
	case kbps <= 63:
		return uint8(kbps), 0 // 1 kbps steps
	case kbps < 576:
		return uint8(64 + (kbps-64)/8), 0 // 64-568 kbps, 8 kbps steps
	case kbps < 8700:
		return uint8(128 + (kbps-576)/64), 0 // 576-8640 kbps, 64 kbps steps
	}

	// Octet 3 reads 0xFE whenever octet 5 carries the value.
	return 0xFE, encodeAPNAMBRExtended(kbps)
}

// encodeAPNAMBRExtended encodes a rate above 8640 kbps and at most 256 Mbps into
// the extended octet (TS 24.301 §9.9.4.2, octet 5).
func encodeAPNAMBRExtended(kbps uint64) uint8 {
	switch {
	case kbps < 17_000:
		return uint8(min((kbps-8600)/100, 0x4A)) // 8700-16000 kbps, 100 kbps steps
	case kbps < 130_000:
		return uint8(min(0x4A+kbps/1000-16, 0xBA)) // 17-128 Mbps, 1 Mbps steps
	default:
		return uint8(min(0xBA+(kbps/1000-128)/2, 0xFA)) // 130-256 Mbps, 2 Mbps steps
	}
}

// decodeAPNAMBRDirection decodes one direction's base, extended and extended-2
// octets to bits per second.
func decodeAPNAMBRDirection(base, ext, ext2 uint8) uint64 {
	// "This value shall be interpreted as '00000000'" (TS 24.301 §9.9.4.2).
	if ext2 == 0xFF {
		ext2 = 0
	}

	return decodeAPNAMBRBase(base, ext) + uint64(ext2)*apnAMBRExtended2Step
}

// decodeAPNAMBRBase decodes the base and extended octets, the extended one
// taking precedence when it is not "use the octet below".
func decodeAPNAMBRBase(base, ext uint8) uint64 {
	if ext != 0 {
		return decodeAPNAMBRExtended(ext)
	}

	switch {
	case base == 0x00 || base == 0xFF:
		return 0
	case base <= 0x3F:
		return uint64(base) * 1000
	case base <= 0x7F:
		return (64 + uint64(base-64)*8) * 1000
	default: // 0x80-0xFE
		return (576 + uint64(base-128)*64) * 1000
	}
}

// decodeAPNAMBRExtended decodes a non-zero extended octet (octet 5/6) to bits/s.
func decodeAPNAMBRExtended(ext uint8) uint64 {
	switch {
	case ext <= 0x4A:
		return (8600 + uint64(ext)*100) * 1000 // 8700-16000 kbps
	case ext <= 0xBA:
		return (16 + uint64(ext-0x4A)) * 1_000_000 // 17-128 Mbps
	default: // "All other values shall be interpreted as '11111010'"
		if ext > 0xFA {
			ext = 0xFA
		}

		return (128 + uint64(ext-0xBA)*2) * 1_000_000 // 130-256 Mbps
	}
}

// remainder reads the octets left in r, or nil when there are none. A trailing
// tail of zero octets is absent rather than present-and-empty, so an element
// without one survives a MarshalBinary/Parse round trip unchanged.
func remainder(r *nas.Reader) ([]byte, error) {
	if r.Remaining() == 0 {
		return nil, nil
	}

	return r.Bytes(r.Remaining())
}
