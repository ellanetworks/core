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
// message's LV fields. References are to TS 24.301.

// PDN type values (TS 24.301).
const (
	PDNTypeIPv4   uint8 = 1
	PDNTypeIPv6   uint8 = 2
	PDNTypeIPv4v6 uint8 = 3
)

// PDNAddress is the PDN address: the IP assigned to the UE. IPv4 is
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

// EPSQoS is the EPS quality of service. For a non-GBR default bearer
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

// MarshalAPN encodes a dot-separated APN into labels (TS 23.003): each label
// is a 1-octet length followed by its characters.
func MarshalAPN(apn string) ([]byte, error) {
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

// ParseAPN decodes a labelled APN value part into its dot-separated form.
func ParseAPN(b []byte) (string, error) {
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

// APNAMBR is the APN aggregate maximum bit rate. DownlinkOctet and
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

// APNAMBRFromBitsPerSecond builds an APN-AMBR from downlink/uplink rates (bits
// per second); it is the inverse of BitsPerSecond, and Marshal then renders the
// wire octets per TS 24.301 (TS 24.008): the base octet
// (≤8640 kbps), the extended octet (octets 5/6, ≤256 Mbps), and the extended-2
// octet (octets 7/8, up to 10 Gbps) for higher rates.
func APNAMBRFromBitsPerSecond(downlinkBps, uplinkBps uint64) APNAMBR {
	dlBase, dlExt, dlExt2 := encodeAPNAMBROctet(downlinkBps)
	ulBase, ulExt, ulExt2 := encodeAPNAMBROctet(uplinkBps)

	a := APNAMBR{DownlinkOctet: dlBase, UplinkOctet: ulBase}

	// Octets are present together per pair; a higher octet pair implies
	// the lower one. A direction that does not need an extension carries 0x00,
	// meaning "use the lower octet".
	switch {
	case dlExt2 != 0 || ulExt2 != 0:
		a.Extended = []byte{dlExt, ulExt, dlExt2, ulExt2}
	case dlExt != 0 || ulExt != 0:
		a.Extended = []byte{dlExt, ulExt}
	}

	return a
}

// BitsPerSecond decodes an APN-AMBR into downlink/uplink rates (bits per second).
func (a APNAMBR) BitsPerSecond() (downlink, uplink uint64) {
	var dlExt, ulExt, dlExt2, ulExt2 uint8

	if len(a.Extended) >= 2 {
		dlExt, ulExt = a.Extended[0], a.Extended[1]
	}

	if len(a.Extended) >= 4 {
		dlExt2, ulExt2 = a.Extended[2], a.Extended[3]
	}

	return decodeAPNAMBROctet(a.DownlinkOctet, dlExt, dlExt2), decodeAPNAMBROctet(a.UplinkOctet, ulExt, ulExt2)
}

// encodeAPNAMBROctet returns the base octet (octet 3/4), extended octet (octet
// 5/6), and extended-2 octet (octet 7/8) for one direction. Above 8640 kbps the
// base octet is 0xFE; above 256 Mbps the extended octet is 0xFA and the value is
// carried in the extended-2 octet.
func encodeAPNAMBROctet(bps uint64) (base, ext, ext2 uint8) {
	kbps := bps / 1000

	switch {
	case kbps == 0:
		return 0xFF, 0, 0 // 0 kbps
	case kbps <= 63:
		return uint8(kbps), 0, 0 // 1-63 kbps, 1 kbps granularity
	case kbps <= 568:
		return uint8(64 + (kbps-64)/8), 0, 0 // 64-568 kbps, 8 kbps granularity
	case kbps <= 8640:
		return uint8(128 + (kbps-576)/64), 0, 0 // 576-8640 kbps, 64 kbps granularity
	default:
		ext, ext2 = encodeAPNAMBRExtended(kbps)

		return 0xFE, ext, ext2
	}
}

// encodeAPNAMBRExtended returns the extended octet (octet 5/6) and extended-2
// octet (octet 7/8) for rates above 8640 kbps (TS 24.008).
func encodeAPNAMBRExtended(kbps uint64) (ext, ext2 uint8) {
	mbps := kbps / 1000

	switch {
	case kbps <= 16000:
		return uint8((kbps - 8600) / 100), 0 // 8700-16000 kbps, 100 kbps granularity
	case mbps <= 128:
		return uint8(74 + (mbps - 16)), 0 // 17-128 Mbps, 1 Mbps granularity
	case mbps <= 256:
		return uint8(186 + (mbps-128)/2), 0 // 130-256 Mbps, 2 Mbps granularity
	default:
		// Above 256 Mbps the extended octet is 0xFA and the value is in octet 7/8.
		return 0xFA, encodeAPNAMBRExtended2(mbps)
	}
}

// encodeAPNAMBRExtended2 returns the extended-2 octet (octet 7/8) for rates above
// 256 Mbps (TS 24.008).
func encodeAPNAMBRExtended2(mbps uint64) uint8 {
	switch {
	case mbps <= 500:
		return uint8((mbps - 256) / 4) // 260-500 Mbps, 4 Mbps granularity
	case mbps <= 1500:
		return uint8(0x3D + (mbps-500)/10) // 510-1500 Mbps, 10 Mbps granularity
	case mbps <= 10000:
		return uint8(0xA1 + (mbps-1500)/100) // 1600-10000 Mbps, 100 Mbps granularity
	default:
		return 0xF6 // clamp at 10 Gbps
	}
}

// decodeAPNAMBROctet decodes one direction's base + extended + extended-2 octets
// to bits/s. A non-zero higher octet takes precedence.
func decodeAPNAMBROctet(base, ext, ext2 uint8) uint64 {
	if ext2 != 0 {
		return decodeAPNAMBRExtended2(ext2)
	}

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
	default: // 187-250 (and the marker value 0xFA)
		return (128 + uint64(ext-186)*2) * 1_000_000 // 130-256 Mbps
	}
}

// decodeAPNAMBRExtended2 decodes a non-zero extended-2 octet (octet 7/8) to
// bits/s (TS 24.008).
func decodeAPNAMBRExtended2(ext2 uint8) uint64 {
	switch {
	case ext2 <= 0x3D:
		return (256 + uint64(ext2)*4) * 1_000_000 // 260-500 Mbps
	case ext2 <= 0xA1:
		return (500 + uint64(ext2-0x3D)*10) * 1_000_000 // 510-1500 Mbps
	default:
		return (1500 + uint64(ext2-0xA1)*100) * 1_000_000 // 1600-10000 Mbps
	}
}

// TAIList is a tracking area identity list of list type "00" — one
// PLMN with one or more TACs, the form an MME emits in ATTACH ACCEPT. Other list
// types are rejected on decode.
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

// UENetworkCapability is the UE network capability. EEA and EIA are
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
