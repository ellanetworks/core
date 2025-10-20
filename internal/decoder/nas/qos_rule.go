package nas

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ellanetworks/core/internal/decoder/utils"
)

type PacketFilterComponent struct {
	ComponentValue []byte                 `json:"component_value"`
	ComponentType  utils.EnumField[uint8] `json:"component_type"`
}

type PacketFilter struct {
	Content       []PacketFilterComponent `json:"content"`
	Direction     utils.EnumField[uint8]  `json:"direction"`
	Identifier    uint8                   `json:"identifier"` // only 0-15
	ContentLength uint8                   `json:"content_length"`
}

type QosRule struct {
	PacketFilterList []PacketFilter         `json:"packet_filter_list"`
	Identifier       uint8                  `json:"identifier"`
	OperationCode    uint8                  `json:"operation_code"`
	DQR              utils.EnumField[uint8] `json:"dqr"`
	Segregation      uint8                  `json:"segregation"`
	Precedence       uint8                  `json:"precedence"`
	QFI              uint8                  `json:"qfi"`
	Length           uint8                  `json:"length"`
}

func dqrToEnum(dqr uint8) utils.EnumField[uint8] {
	switch dqr & 0x01 {
	case 1:
		return utils.MakeEnum(dqr&0x01, "default", false)
	case 0:
		return utils.MakeEnum(dqr&0x01, "non-default", false)
	default:
		return utils.MakeEnum(dqr&0x01, "", true)
	}
}

func buildPFComponentTypeString(ct uint8) utils.EnumField[uint8] {
	switch ct {
	case 0x01:
		return utils.MakeEnum(ct, "MatchAll", false)
	case 0x10:
		return utils.MakeEnum(ct, "IPv4RemoteAddress", false)
	case 0x11:
		return utils.MakeEnum(ct, "IPv4LocalAddress", false)
	case 0x21:
		return utils.MakeEnum(ct, "IPv6RemoteAddress", false)
	case 0x23:
		return utils.MakeEnum(ct, "IPv6LocalAddress", false)
	case 0x30:
		return utils.MakeEnum(ct, "ProtocolIdentifierOrNextHeader", false)
	case 0x40:
		return utils.MakeEnum(ct, "SingleLocalPort", false)
	case 0x41:
		return utils.MakeEnum(ct, "LocalPortRange", false)
	case 0x50:
		return utils.MakeEnum(ct, "SingleRemotePort", false)
	case 0x51:
		return utils.MakeEnum(ct, "RemotePortRange", false)
	case 0x60:
		return utils.MakeEnum(ct, "SecurityParameterIndex", false)
	case 0x70:
		return utils.MakeEnum(ct, "TypeOfServiceOrTrafficClass", false)
	case 0x80:
		return utils.MakeEnum(ct, "FlowLabel", false)
	case 0x81:
		return utils.MakeEnum(ct, "DestinationMACAddress", false)
	case 0x82:
		return utils.MakeEnum(ct, "SourceMACAddress", false)
	case 0x87:
		return utils.MakeEnum(ct, "Ethertype", false)
	default:
		return utils.MakeEnum(ct, fmt.Sprintf("Unknown(0x%02X)", ct), true)
	}
}

func buildPFDirectionString(n uint8) utils.EnumField[uint8] {
	switch n & 0x0F {
	case 0x01:
		return utils.MakeEnum(n&0x0F, "downlink", false)
	case 0x02:
		return utils.MakeEnum(n&0x0F, "uplink", false)
	case 0x03:
		return utils.MakeEnum(n&0x0F, "bidirectional", false)
	default:
		return utils.MakeEnum(n&0x0F, "", true)
	}
}

func pfComponentValueLen(t uint8) (int, bool) {
	switch t {
	case 0x01: // MatchAll
		return 0, true
	case 0x30: // ProtocolIdentifierOrNextHeader
		return 1, true
	case 0x40, 0x50: // SingleLocalPort / SingleRemotePort
		return 2, true
	case 0x41, 0x51: // LocalPortRange / RemotePortRange
		return 4, true
	case 0x60: // SecurityParameterIndex
		return 4, true
	case 0x70: // TypeOfServiceOrTrafficClass (value+mask)
		return 2, true
	case 0x80: // IPv6 Flow Label (20 bits → 3 bytes)
		return 3, true
	case 0x10, 0x11: // IPv4 addr+mask
		return 8, true
	case 0x21, 0x23: // IPv6 addr+mask
		return 32, true
	case 0x81, 0x82: // Dest/Source MAC + mask
		return 12, true
	case 0x87: // Ethertype
		return 2, true
	default:
		return 0, false
	}
}

func unmarshalQosRule(b []byte) (QosRule, int, error) {
	var r QosRule
	if len(b) < 3 {
		return r, 0, io.ErrUnexpectedEOF
	}
	cur := 0

	// Identifier
	r.Identifier = b[cur]
	cur++

	// Content length (2B, BE)
	if len(b[cur:]) < 2 {
		return r, 0, io.ErrUnexpectedEOF
	}
	contentLen := int(binary.BigEndian.Uint16(b[cur : cur+2]))
	cur += 2

	if contentLen > 0xFF {
		return r, 0, fmt.Errorf("qos rule content length %d exceeds uint8 field; change QosRule.Length to uint16", contentLen)
	}
	r.Length = uint8(contentLen)

	if len(b[cur:]) < contentLen {
		return r, 0, io.ErrUnexpectedEOF
	}
	content := b[cur : cur+contentLen]
	cc := 0

	// Header: Op(3) | DQR(1) | numPF(4)
	if len(content) < 1 {
		return r, 0, io.ErrUnexpectedEOF
	}
	hdr := content[cc]
	cc++

	r.OperationCode = (hdr >> 5) & 0x07
	r.DQR = dqrToEnum((hdr >> 4) & 0x01)
	numPF := int(hdr & 0x0F)

	// Packet filters
	r.PacketFilterList = r.PacketFilterList[:0]
	for i := 0; i < numPF; i++ {
		if cc >= len(content) {
			return r, 0, io.ErrUnexpectedEOF
		}
		pf, n, err := unmarshalPacketFilter(content[cc:])
		if err != nil {
			return r, 0, fmt.Errorf("packet filter %d: %w", i, err)
		}
		if n <= 0 {
			return r, 0, fmt.Errorf("packet filter %d consumed 0 bytes", i)
		}
		r.PacketFilterList = append(r.PacketFilterList, pf)
		cc += n
	}

	// Precedence + Segregation|QFI
	if len(content[cc:]) < 2 {
		return r, 0, io.ErrUnexpectedEOF
	}
	r.Precedence = content[cc]
	cc++
	segQFI := content[cc]
	cc++
	r.Segregation = segQFI >> 6
	r.QFI = segQFI & 0x3F

	if cc != contentLen {
		return r, 0, fmt.Errorf("qos rule length mismatch: contentLen=%d consumed=%d", contentLen, cc)
	}

	total := cur + contentLen
	return r, total, nil
}

func UnmarshalQosRules(data []byte) ([]QosRule, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var out []QosRule

	cur := 0

	for cur < len(data) {
		rule, n, err := unmarshalQosRule(data[cur:])
		if err != nil {
			return nil, fmt.Errorf("qos rule at offset %d: %w", cur, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("decoder consumed 0 bytes at offset %d", cur)
		}
		out = append(out, rule)
		cur += n
	}
	return out, nil
}

func unmarshalPacketFilter(b []byte) (PacketFilter, int, error) {
	var pf PacketFilter
	if len(b) < 2 {
		return pf, 0, io.ErrUnexpectedEOF
	}
	cur := 0

	// Header: Direction (high nibble) | Identifier (low nibble)
	h := b[cur]
	cur++

	dirNibble := (h >> 4) & 0x0F
	idNibble := h & 0x0F

	pf.Direction = buildPFDirectionString(dirNibble)
	pf.Identifier = idNibble

	// Content length
	pf.ContentLength = b[cur]
	cur++

	if len(b[cur:]) < int(pf.ContentLength) {
		return pf, 0, io.ErrUnexpectedEOF
	}
	content := b[cur : cur+int(pf.ContentLength)]
	cur += int(pf.ContentLength)

	// Parse components from content
	var comps []PacketFilterComponent
	i := 0
	for i < len(content) {
		t := content[i]
		i++

		valLen, known := pfComponentValueLen(t)
		if !known {
			// Unknown type → preserve the rest opaquely starting at this type byte
			i-- // include t itself
			comps = append(comps, PacketFilterComponent{
				ComponentType:  utils.MakeEnum(t, "", true),
				ComponentValue: append([]byte(nil), content[i:]...),
			})
			break
		}

		if valLen == 0 {
			comps = append(comps, PacketFilterComponent{
				ComponentType:  buildPFComponentTypeString(t),
				ComponentValue: nil,
			})
			continue
		}

		if i+valLen > len(content) {
			return pf, 0, io.ErrUnexpectedEOF
		}
		val := content[i : i+valLen]
		i += valLen

		comps = append(comps, PacketFilterComponent{
			ComponentType:  buildPFComponentTypeString(t),
			ComponentValue: append([]byte(nil), val...),
		})
	}

	pf.Content = comps
	return pf, cur, nil
}
