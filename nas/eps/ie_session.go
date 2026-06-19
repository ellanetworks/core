// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/nas/common"
)

// These IE codecs produce/consume the *value part* of an information element
// (the bytes inside its LV/LV-E length). The MME composes them into the
// message's LV fields. References are to TS 24.301 §9.9.

// PDN type values (TS 24.301 §9.9.4.10).
const (
	PDNTypeIPv4   uint8 = 1
	PDNTypeIPv6   uint8 = 2
	PDNTypeIPv4v6 uint8 = 3
)

// PDNAddress is the PDN address (§9.9.4.9): the IP assigned to the UE. IPv4 is
// the 4-octet address; IPv6IID the 8-octet interface identifier.
type PDNAddress struct {
	PDNType uint8
	IPv4    [4]byte
	IPv6IID [8]byte
}

// Marshal returns the PDN address value part.
func (a PDNAddress) Marshal() []byte {
	var w common.Writer

	w.U8(a.PDNType & 0x07)

	switch a.PDNType {
	case PDNTypeIPv4:
		w.Raw(a.IPv4[:])
	case PDNTypeIPv6:
		w.Raw(a.IPv6IID[:])
	case PDNTypeIPv4v6:
		w.Raw(a.IPv6IID[:])
		w.Raw(a.IPv4[:])
	}

	return w.Bytes()
}

// ParsePDNAddress decodes a PDN address value part.
func ParsePDNAddress(b []byte) (PDNAddress, error) {
	r := common.NewReader(b)

	t, err := r.U8()
	if err != nil {
		return PDNAddress{}, err
	}

	a := PDNAddress{PDNType: t & 0x07}

	switch a.PDNType {
	case PDNTypeIPv4:
		v, err := r.Bytes(4)
		if err != nil {
			return a, err
		}

		copy(a.IPv4[:], v)
	case PDNTypeIPv6:
		v, err := r.Bytes(8)
		if err != nil {
			return a, err
		}

		copy(a.IPv6IID[:], v)
	case PDNTypeIPv4v6:
		v6, err := r.Bytes(8)
		if err != nil {
			return a, err
		}

		copy(a.IPv6IID[:], v6)

		v4, err := r.Bytes(4)
		if err != nil {
			return a, err
		}

		copy(a.IPv4[:], v4)
	default:
		return a, fmt.Errorf("nas/eps: unsupported PDN type %d", a.PDNType)
	}

	return a, nil
}

// EPSQoS is the EPS quality of service (§9.9.4.3). For a non-GBR default bearer
// only the QCI is present; BitRates holds the optional MBR/GBR octets.
type EPSQoS struct {
	QCI      uint8
	BitRates []byte
}

// Marshal returns the EPS QoS value part.
func (q EPSQoS) Marshal() []byte {
	var w common.Writer

	w.U8(q.QCI)
	w.Raw(q.BitRates)

	return w.Bytes()
}

// ParseEPSQoS decodes an EPS QoS value part.
func ParseEPSQoS(b []byte) (EPSQoS, error) {
	r := common.NewReader(b)

	qci, err := r.U8()
	if err != nil {
		return EPSQoS{}, err
	}

	rest, err := r.Bytes(r.Remaining())
	if err != nil {
		return EPSQoS{}, err
	}

	return EPSQoS{QCI: qci, BitRates: rest}, nil
}

// EncodeAPN encodes a dot-separated APN into labels (TS 23.003 §9.1): each label
// is a 1-octet length followed by its characters.
func EncodeAPN(apn string) ([]byte, error) {
	var w common.Writer

	for _, label := range strings.Split(apn, ".") {
		if len(label) > 0xFF {
			return nil, fmt.Errorf("nas/eps: APN label exceeds 255 octets")
		}

		w.U8(uint8(len(label)))
		w.Raw([]byte(label))
	}

	return w.Bytes(), nil
}

// DecodeAPN decodes a labelled APN value part into its dot-separated form.
func DecodeAPN(b []byte) (string, error) {
	r := common.NewReader(b)

	var labels []string

	for r.Remaining() > 0 {
		label, err := r.LV()
		if err != nil {
			return "", err
		}

		labels = append(labels, string(label))
	}

	return strings.Join(labels, "."), nil
}

// APNAMBR is the APN aggregate maximum bit rate (§9.9.4.2). DownlinkOctet and
// UplinkOctet use the GPRS bit-rate coding; Extended holds the optional
// higher-rate octets.
type APNAMBR struct {
	DownlinkOctet uint8
	UplinkOctet   uint8
	Extended      []byte
}

// Marshal returns the APN-AMBR value part.
func (a APNAMBR) Marshal() []byte {
	var w common.Writer

	w.U8(a.DownlinkOctet)
	w.U8(a.UplinkOctet)
	w.Raw(a.Extended)

	return w.Bytes()
}

// ParseAPNAMBR decodes an APN-AMBR value part.
func ParseAPNAMBR(b []byte) (APNAMBR, error) {
	r := common.NewReader(b)

	dl, err := r.U8()
	if err != nil {
		return APNAMBR{}, err
	}

	ul, err := r.U8()
	if err != nil {
		return APNAMBR{}, err
	}

	ext, err := r.Bytes(r.Remaining())
	if err != nil {
		return APNAMBR{}, err
	}

	return APNAMBR{DownlinkOctet: dl, UplinkOctet: ul, Extended: ext}, nil
}

// EncodeAPNAMBR encodes downlink/uplink rates (bits per second) into an APN-AMBR
// IE value per TS 24.301 §9.9.4.2, using the base octet (≤8640 kbps) and the
// extended octet (≤256 Mbps). Rates above 256 Mbps are clamped to 256 Mbps in
// the encoded value (the extended-2 octets, octets 7-8, are not emitted); the
// data-plane enforcement of the exact configured rate happens elsewhere (the UPF
// QER) and is unaffected.
func EncodeAPNAMBR(downlinkBps, uplinkBps uint64) APNAMBR {
	dlBase, dlExt := encodeAPNAMBROctet(downlinkBps)
	ulBase, ulExt := encodeAPNAMBROctet(uplinkBps)

	a := APNAMBR{DownlinkOctet: dlBase, UplinkOctet: ulBase}

	// Octets 5 (DL ext) and 6 (UL ext) are present together or not at all
	// (§9.9.4.2). A side that needs no extension carries 0x00 there, meaning
	// "use the base octet".
	if dlExt != 0 || ulExt != 0 {
		a.Extended = []byte{dlExt, ulExt}
	}

	return a
}

// BitsPerSecond decodes an APN-AMBR into downlink/uplink rates (bits per second).
func (a APNAMBR) BitsPerSecond() (downlink, uplink uint64) {
	var dlExt, ulExt uint8
	if len(a.Extended) >= 2 {
		dlExt, ulExt = a.Extended[0], a.Extended[1]
	}

	return decodeAPNAMBROctet(a.DownlinkOctet, dlExt), decodeAPNAMBROctet(a.UplinkOctet, ulExt)
}

// encodeAPNAMBROctet returns the base octet (octet 3/4) and extended octet
// (octet 5/6) for one direction. When the rate exceeds 8640 kbps the base octet
// is set to 0xFE ("8640 kbps") and the value is carried in the extended octet.
func encodeAPNAMBROctet(bps uint64) (base, ext uint8) {
	kbps := bps / 1000

	switch {
	case kbps == 0:
		return 0xFF, 0 // 0 kbps
	case kbps <= 63:
		return uint8(kbps), 0 // 1-63 kbps, 1 kbps granularity
	case kbps <= 568:
		return uint8(64 + (kbps-64)/8), 0 // 64-568 kbps, 8 kbps granularity
	case kbps <= 8640:
		return uint8(128 + (kbps-576)/64), 0 // 576-8640 kbps, 64 kbps granularity
	default:
		return 0xFE, encodeAPNAMBRExtended(kbps)
	}
}

// encodeAPNAMBRExtended returns the extended octet (octet 5/6) for rates above
// 8640 kbps, clamped at 256 Mbps (0xFA).
func encodeAPNAMBRExtended(kbps uint64) uint8 {
	mbps := kbps / 1000

	switch {
	case kbps <= 16000:
		return uint8((kbps - 8600) / 100) // 8700-16000 kbps, 100 kbps granularity
	case mbps <= 128:
		return uint8(74 + (mbps - 16)) // 17-128 Mbps, 1 Mbps granularity
	case mbps <= 256:
		return uint8(186 + (mbps-128)/2) // 130-256 Mbps, 2 Mbps granularity
	default:
		return 0xFA // clamp at 256 Mbps
	}
}

// decodeAPNAMBROctet decodes one direction's base + extended octets to bits/s.
func decodeAPNAMBROctet(base, ext uint8) uint64 {
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
	case ext <= 74:
		return (8600 + uint64(ext)*100) * 1000 // 8700-16000 kbps
	case ext <= 186:
		return (16 + uint64(ext-74)) * 1_000_000 // 17-128 Mbps
	default: // 187-250 (and the clamp value 0xFA)
		return (128 + uint64(ext-186)*2) * 1_000_000 // 130-256 Mbps
	}
}

// TAIList is a tracking area identity list (§9.9.3.33) of list type "00" — one
// PLMN with one or more TACs, the form an MME emits in ATTACH ACCEPT. Other list
// types are rejected on decode (deferred).
type TAIList struct {
	MCC, MNC string
	TACs     []uint16
}

// Marshal returns the TAI-list value part.
func (l TAIList) Marshal() ([]byte, error) {
	if len(l.TACs) < 1 || len(l.TACs) > 16 {
		return nil, fmt.Errorf("nas/eps: TAI list has %d TACs, want 1..16", len(l.TACs))
	}

	plmn, err := common.EncodePLMN(l.MCC, l.MNC)
	if err != nil {
		return nil, err
	}

	var w common.Writer

	w.U8(uint8(len(l.TACs) - 1)) // type "00" (bits 7-6 = 0) | number of elements - 1
	w.Raw(plmn[:])

	for _, tac := range l.TACs {
		w.U16(tac)
	}

	return w.Bytes(), nil
}

// ParseTAIList decodes a type-"00" TAI-list value part.
func ParseTAIList(b []byte) (TAIList, error) {
	r := common.NewReader(b)

	hdr, err := r.U8()
	if err != nil {
		return TAIList{}, err
	}

	if listType := hdr >> 5 & 0x03; listType != 0 {
		return TAIList{}, fmt.Errorf("nas/eps: TAI list type %d not supported", listType)
	}

	n := int(hdr&0x1F) + 1

	plmn, err := r.Bytes(3)
	if err != nil {
		return TAIList{}, err
	}

	mcc, mnc := common.DecodePLMN([3]byte{plmn[0], plmn[1], plmn[2]})
	l := TAIList{MCC: mcc, MNC: mnc, TACs: make([]uint16, 0, n)}

	for i := 0; i < n; i++ {
		tac, err := r.U16()
		if err != nil {
			return TAIList{}, err
		}

		l.TACs = append(l.TACs, tac)
	}

	return l, nil
}

// UENetworkCapability is the UE network capability (§9.9.3.34). EEA and EIA are
// the EPS encryption / integrity algorithm bitmaps; Rest holds the remaining
// (UMTS/extended) octets.
type UENetworkCapability struct {
	EEA  uint8
	EIA  uint8
	Rest []byte
}

// Marshal returns the UE network capability value part.
func (c UENetworkCapability) Marshal() []byte {
	var w common.Writer

	w.U8(c.EEA)
	w.U8(c.EIA)
	w.Raw(c.Rest)

	return w.Bytes()
}

// ParseUENetworkCapability decodes a UE network capability value part.
func ParseUENetworkCapability(b []byte) (UENetworkCapability, error) {
	r := common.NewReader(b)

	eea, err := r.U8()
	if err != nil {
		return UENetworkCapability{}, err
	}

	eia, err := r.U8()
	if err != nil {
		return UENetworkCapability{}, err
	}

	rest, err := r.Bytes(r.Remaining())
	if err != nil {
		return UENetworkCapability{}, err
	}

	return UENetworkCapability{EEA: eea, EIA: eia, Rest: rest}, nil
}

// SupportsEEA reports whether the UE supports 128-EEAn (n = 0..7).
func (c UENetworkCapability) SupportsEEA(n uint8) bool {
	return n <= 7 && c.EEA&(1<<(7-n)) != 0
}

// SupportsEIA reports whether the UE supports 128-EIAn (n = 0..7).
func (c UENetworkCapability) SupportsEIA(n uint8) bool {
	return n <= 7 && c.EIA&(1<<(7-n)) != 0
}
