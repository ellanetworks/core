// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// The configuration protocol identifiers TS 24.008 §10.5.6.3 requires be
// supported, whichever access carries the element. Their contents are PPP
// packets stripped of the Protocol and Padding octets: LCP and IPCP are
// specified in RFC 1661 and RFC 1332, PAP in RFC 1334 and CHAP in RFC 1994.
const (
	PCOProtocolIPCP uint16 = 0x8021
	PCOProtocolLCP  uint16 = 0xc021
	PCOProtocolPAP  uint16 = 0xc023
	PCOProtocolCHAP uint16 = 0xc223
)

const (
	pppConfigureRequest uint8 = 1
	pppConfigureAck     uint8 = 2
	pppConfigureNak     uint8 = 3
	pppConfigureReject  uint8 = 4
)

const (
	papAuthenticateRequest uint8 = 1
	papAuthenticateAck     uint8 = 2
)

const (
	chapResponse uint8 = 2
	chapSuccess  uint8 = 3
)

const (
	ipcpOptionIPAddress     uint8 = 3
	ipcpOptionPrimaryDNS    uint8 = 129
	ipcpOptionPrimaryNBNS   uint8 = 130
	ipcpOptionSecondaryDNS  uint8 = 131
	ipcpOptionSecondaryNBNS uint8 = 132
)

const (
	pppHeaderLen    = 4
	pppOptionHdrLen = 2
	ipv4Len         = 4
)

type pppPacket struct {
	code       uint8
	identifier uint8
	data       []byte
}

func parsePPPPacket(b []byte) (pppPacket, error) {
	if len(b) < pppHeaderLen {
		return pppPacket{}, fmt.Errorf("nas: ppp packet is %d octets, want at least %d", len(b), pppHeaderLen)
	}

	length := int(binary.BigEndian.Uint16(b[2:]))
	if length < pppHeaderLen || length > len(b) {
		return pppPacket{}, fmt.Errorf(
			"nas: ppp packet length field is %d, want between %d and %d", length, pppHeaderLen, len(b))
	}

	return pppPacket{code: b[0], identifier: b[1], data: b[pppHeaderLen:length]}, nil
}

func (p pppPacket) marshal() ([]byte, error) {
	total := pppHeaderLen + len(p.data)
	if total > maxOneOctetContainerLen {
		return nil, fmt.Errorf("nas: ppp packet is %d octets, want at most %d", total, maxOneOctetContainerLen)
	}

	out := make([]byte, pppHeaderLen, total)
	out[0] = p.code
	out[1] = p.identifier
	binary.BigEndian.PutUint16(out[2:], uint16(total))

	return append(out, p.data...), nil
}

type pppOption struct {
	typ  uint8
	data []byte
}

func parsePPPOptions(b []byte) ([]pppOption, error) {
	var out []pppOption

	for len(b) > 0 {
		if len(b) < pppOptionHdrLen {
			return nil, fmt.Errorf("nas: ppp option header is %d octets, want %d", len(b), pppOptionHdrLen)
		}

		length := int(b[1])
		if length < pppOptionHdrLen || length > len(b) {
			return nil, fmt.Errorf(
				"nas: ppp option length is %d, want between %d and %d", length, pppOptionHdrLen, len(b))
		}

		out = append(out, pppOption{typ: b[0], data: b[pppOptionHdrLen:length]})
		b = b[length:]
	}

	return out, nil
}

func appendPPPOption(b []byte, o pppOption) []byte {
	b = append(b, o.typ, uint8(len(o.data)+pppOptionHdrLen))

	return append(b, o.data...)
}

// AnswerProtocolOptions answers the configuration protocol options list of an
// MS-to-network element. dns is the resolver offered over IPCP and ueIPv4 the
// address already allocated to the session; an invalid address withdraws the
// matching IPCP option from negotiation. A unit that is malformed, or that
// carries a code with no defined answer, is left unanswered rather than
// failing the session.
//
// The result belongs at the head of the reply — see
// [ProtocolConfigurationOptions.PrependProtocolOptions].
func AnswerProtocolOptions(requests []PCOContainer, dns, ueIPv4 netip.Addr) []PCOContainer {
	dns = dns.Unmap()
	ueIPv4 = ueIPv4.Unmap()

	var out []PCOContainer

	for _, c := range requests {
		content, err := answerProtocolOption(c, dns, ueIPv4)
		if err != nil || content == nil {
			continue
		}

		out = append(out, PCOContainer{ID: c.ID, Content: content})
	}

	return out
}

func answerProtocolOption(c PCOContainer, dns, ueIPv4 netip.Addr) ([]byte, error) {
	p, err := parsePPPPacket(c.Content)
	if err != nil {
		return nil, err
	}

	switch c.ID {
	case PCOProtocolIPCP:
		if p.code != pppConfigureRequest {
			return nil, nil
		}

		return answerIPCP(p, dns, ueIPv4)
	case PCOProtocolLCP:
		if p.code != pppConfigureRequest {
			return nil, nil
		}

		return answerLCP(p)
	case PCOProtocolPAP:
		if p.code != papAuthenticateRequest {
			return nil, nil
		}

		return pppPacket{code: papAuthenticateAck, identifier: p.identifier, data: []byte{0}}.marshal()
	case PCOProtocolCHAP:
		if p.code != chapResponse {
			return nil, nil
		}

		return pppPacket{code: chapSuccess, identifier: p.identifier}.marshal()
	default:
		return nil, nil
	}
}

func answerIPCP(req pppPacket, dns, ueIPv4 netip.Addr) ([]byte, error) {
	options, err := parsePPPOptions(req.data)
	if err != nil {
		return nil, err
	}

	var rejected, naked []byte

	for _, o := range options {
		want, ok := ipcpOptionAddress(o.typ, dns, ueIPv4)
		if !ok || len(o.data) != ipv4Len {
			rejected = appendPPPOption(rejected, o)

			continue
		}

		if netip.AddrFrom4([ipv4Len]byte(o.data)) == want {
			continue
		}

		v := want.As4()
		naked = appendPPPOption(naked, pppOption{typ: o.typ, data: v[:]})
	}

	switch {
	case len(rejected) > 0:
		return pppPacket{code: pppConfigureReject, identifier: req.identifier, data: rejected}.marshal()
	case len(naked) > 0:
		return pppPacket{code: pppConfigureNak, identifier: req.identifier, data: naked}.marshal()
	default:
		return pppPacket{code: pppConfigureAck, identifier: req.identifier, data: req.data}.marshal()
	}
}

func ipcpOptionAddress(typ uint8, dns, ueIPv4 netip.Addr) (netip.Addr, bool) {
	switch typ {
	case ipcpOptionPrimaryDNS, ipcpOptionSecondaryDNS:
		return dns, dns.Is4()
	case ipcpOptionIPAddress:
		return ueIPv4, ueIPv4.Is4()
	default:
		return netip.Addr{}, false
	}
}

func answerLCP(req pppPacket) ([]byte, error) {
	options, err := parsePPPOptions(req.data)
	if err != nil {
		return nil, err
	}

	if len(options) == 0 {
		return pppPacket{code: pppConfigureAck, identifier: req.identifier}.marshal()
	}

	return pppPacket{code: pppConfigureReject, identifier: req.identifier, data: req.data}.marshal()
}

// PCOProtocolOptionSummary renders the contents of a configuration protocol
// options unit for display: the PPP code, and for IPCP the options it carries.
// It returns the empty string for a unit it cannot read, leaving the caller to
// fall back to the raw octets.
func PCOProtocolOptionSummary(id uint16, content []byte) string {
	p, err := parsePPPPacket(content)
	if err != nil {
		return ""
	}

	code := pppCodeName(id, p.code)
	if code == "" {
		return ""
	}

	if id != PCOProtocolIPCP {
		return code
	}

	options, err := parsePPPOptions(p.data)
	if err != nil || len(options) == 0 {
		return code
	}

	rendered := make([]string, 0, len(options))
	for _, o := range options {
		rendered = append(rendered, ipcpOptionSummary(o))
	}

	return code + ": " + strings.Join(rendered, ", ")
}

func pppCodeName(id uint16, code uint8) string {
	switch id {
	case PCOProtocolIPCP, PCOProtocolLCP:
		switch code {
		case 1:
			return "Configure-Request"
		case 2:
			return "Configure-Ack"
		case 3:
			return "Configure-Nak"
		case 4:
			return "Configure-Reject"
		case 5:
			return "Terminate-Request"
		case 6:
			return "Terminate-Ack"
		case 7:
			return "Code-Reject"
		}
	case PCOProtocolPAP:
		switch code {
		case 1:
			return "Authenticate-Request"
		case 2:
			return "Authenticate-Ack"
		case 3:
			return "Authenticate-Nak"
		}
	case PCOProtocolCHAP:
		switch code {
		case 1:
			return "Challenge"
		case 2:
			return "Response"
		case 3:
			return "Success"
		case 4:
			return "Failure"
		}
	}

	return ""
}

func ipcpOptionSummary(o pppOption) string {
	name := ipcpOptionName(o.typ)
	if name == "" {
		name = "option " + strconv.Itoa(int(o.typ))
	}

	if len(o.data) != ipv4Len {
		return name
	}

	return name + " " + netip.AddrFrom4([ipv4Len]byte(o.data)).String()
}

func ipcpOptionName(typ uint8) string {
	switch typ {
	case ipcpOptionIPAddress:
		return "IP Address"
	case ipcpOptionPrimaryDNS:
		return "Primary DNS Server Address"
	case ipcpOptionPrimaryNBNS:
		return "Primary NBNS Server Address"
	case ipcpOptionSecondaryDNS:
		return "Secondary DNS Server Address"
	case ipcpOptionSecondaryNBNS:
		return "Secondary NBNS Server Address"
	default:
		return ""
	}
}
