// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"net"
	"strings"

	"github.com/ellanetworks/core/nas/common"
)

// SNSSAI is the S-NSSAI IE (TS 24.501 §9.11.2.8): a mandatory SST and an
// optional 3-octet SD.
type SNSSAI struct {
	SST uint8
	SD  *[3]byte
}

func (s SNSSAI) marshalValue() []byte {
	if s.SD == nil {
		return []byte{s.SST}
	}

	return append([]byte{s.SST}, s.SD[:]...)
}

func parseSNSSAI(v []byte) *SNSSAI {
	if len(v) == 0 {
		return nil
	}

	s := &SNSSAI{SST: v[0]}

	if len(v) >= 4 {
		var sd [3]byte

		copy(sd[:], v[1:4])
		s.SD = &sd
	}

	return s
}

// labelsToDNN decodes RFC 1035 labels back to a dotted DNN string.
func labelsToDNN(v []byte) string {
	var labels []string

	for i := 0; i < len(v); {
		n := int(v[i])
		i++

		if i+n > len(v) {
			break
		}

		labels = append(labels, string(v[i:i+n]))
		i += n
	}

	return strings.Join(labels, ".")
}

// PDUAddress is the PDU address IE (TS 24.501 §9.11.4.10). The value is the PDU
// session type value followed by the address: a 4-octet IPv4 address, an 8-octet
// IPv6 interface identifier, or the identifier and address for IPv4v6.
type PDUAddress struct {
	SessionType uint8
	IPv4        [4]byte
	IPv6IID     [8]byte
}

func (a PDUAddress) marshalValue() []byte {
	var w common.Writer

	w.U8(a.SessionType & 0x07)

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		w.Raw(a.IPv4[:])
	case PDUSessionTypeIPv6:
		w.Raw(a.IPv6IID[:])
	case PDUSessionTypeIPv4IPv6:
		w.Raw(a.IPv6IID[:])
		w.Raw(a.IPv4[:])
	}

	return w.Bytes()
}

// parsePDUAddress decodes a PDU address IE value (session type followed by the
// address).
func parsePDUAddress(v []byte) *PDUAddress {
	if len(v) == 0 {
		return nil
	}

	a := &PDUAddress{SessionType: v[0] & 0x07}
	body := v[1:]

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		copy(a.IPv4[:], body)
	case PDUSessionTypeIPv6:
		copy(a.IPv6IID[:], body)
	case PDUSessionTypeIPv4IPv6:
		copy(a.IPv6IID[:], body)

		if len(body) >= 12 {
			copy(a.IPv4[:], body[8:12])
		}
	}

	return a
}

// NewPDUAddressIPv4 builds an IPv4 PDU address IE value from a net.IP.
func NewPDUAddressIPv4(ip net.IP) PDUAddress {
	a := PDUAddress{SessionType: PDUSessionTypeIPv4}
	copy(a.IPv4[:], ip.To4())

	return a
}

// dnnLabels encodes a DNN as RFC 1035 labels: each dot-separated label prefixed
// by its length (TS 23.003, TS 24.501 §9.11.2.1B).
func dnnLabels(dnn string) []byte {
	var w common.Writer

	for _, label := range strings.Split(dnn, ".") {
		w.U8(uint8(len(label)))
		w.Raw([]byte(label))
	}

	return w.Bytes()
}

func writeTV2(w *common.Writer, iei, value uint8) {
	w.U8(iei)
	w.U8(value)
}

func writeTLV(w *common.Writer, iei uint8, value []byte) {
	w.U8(iei)
	w.U8(uint8(len(value)))
	w.Raw(value)
}

func writeTLVE(w *common.Writer, iei uint8, value []byte) {
	w.U8(iei)
	w.U16(uint16(len(value)))
	w.Raw(value)
}
