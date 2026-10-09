// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/nas"
)

// TFTOperation is the TFT operation code (TS 24.008 §10.5.6.12).
type TFTOperation uint8

// TFT operation codes (TS 24.008 table 10.5.162).
const (
	TFTIgnore         TFTOperation = 0
	TFTCreate         TFTOperation = 1
	TFTDeleteExisting TFTOperation = 2
	TFTAddFilters     TFTOperation = 3
	TFTReplaceFilters TFTOperation = 4
	TFTDeleteFilters  TFTOperation = 5
	TFTNoOperation    TFTOperation = 6
)

const (
	tftOperationMax    = 6
	maxTFTPacketFilter = 15
	maxTFTLen          = 255
)

// TFTDirection is a packet filter direction (TS 24.008 §10.5.6.12).
type TFTDirection uint8

// Packet filter directions (TS 24.008 table 10.5.162).
const (
	TFTPreRel7       TFTDirection = 0
	TFTDownlink      TFTDirection = 1
	TFTUplink        TFTDirection = 2
	TFTBidirectional TFTDirection = 3
)

// TFTComponentType is a packet filter component type (TS 24.008 table 10.5.162).
type TFTComponentType uint8

// Packet filter component types (TS 24.008 table 10.5.162).
const (
	TFTIPv4RemoteAddress       TFTComponentType = 0x10
	TFTIPv4LocalAddress        TFTComponentType = 0x11
	TFTIPv6RemoteAddress       TFTComponentType = 0x20
	TFTIPv6RemoteAddressPrefix TFTComponentType = 0x21
	TFTIPv6LocalAddressPrefix  TFTComponentType = 0x23
	TFTProtocolIdentifier      TFTComponentType = 0x30
	TFTSingleLocalPort         TFTComponentType = 0x40
	TFTLocalPortRange          TFTComponentType = 0x41
	TFTSingleRemotePort        TFTComponentType = 0x50
	TFTRemotePortRange         TFTComponentType = 0x51
	TFTSecurityParameterIndex  TFTComponentType = 0x60
	TFTTypeOfService           TFTComponentType = 0x70
	TFTFlowLabel               TFTComponentType = 0x80
)

var tftComponentLen = map[TFTComponentType]int{
	TFTIPv4RemoteAddress:       8,
	TFTIPv4LocalAddress:        8,
	TFTIPv6RemoteAddress:       32,
	TFTIPv6RemoteAddressPrefix: 17,
	TFTIPv6LocalAddressPrefix:  17,
	TFTProtocolIdentifier:      1,
	TFTSingleLocalPort:         2,
	TFTLocalPortRange:          4,
	TFTSingleRemotePort:        2,
	TFTRemotePortRange:         4,
	TFTSecurityParameterIndex:  4,
	TFTTypeOfService:           2,
	TFTFlowLabel:               3,
}

// TFTComponent is a packet filter component and its fixed-length value.
type TFTComponent struct {
	Type  TFTComponentType
	Value []byte
}

// TFTPacketFilter is one packet filter of a TFT.
type TFTPacketFilter struct {
	Identifier uint8
	Direction  TFTDirection
	Precedence uint8
	Components []TFTComponent
}

// TFTParameter is one entry of a TFT parameters list.
type TFTParameter struct {
	Identifier uint8
	Contents   []byte
}

// TrafficFlowTemplate is the traffic flow template IE value (TS 24.008 §10.5.6.12).
type TrafficFlowTemplate struct {
	Operation         TFTOperation
	Filters           []TFTPacketFilter
	DeleteIdentifiers []uint8
	Parameters        []TFTParameter
}

// RemoteAddress is the IPv4 remote address or IPv6 remote address/prefix length component for p.
func RemoteAddress(p netip.Prefix) TFTComponent {
	p = p.Masked()

	if p.Addr().Is4() {
		mask := netip.PrefixFrom(netip.AddrFrom4([4]byte{0xFF, 0xFF, 0xFF, 0xFF}), p.Bits()).Masked().Addr().As4()
		addr := p.Addr().As4()

		return TFTComponent{Type: TFTIPv4RemoteAddress, Value: append(addr[:], mask[:]...)}
	}

	addr := p.Addr().As16()

	return TFTComponent{Type: TFTIPv6RemoteAddressPrefix, Value: append(addr[:], uint8(p.Bits()))}
}

// ProtocolIdentifier is the protocol identifier/next header component.
func ProtocolIdentifier(proto uint8) TFTComponent {
	return TFTComponent{Type: TFTProtocolIdentifier, Value: []byte{proto}}
}

// SingleLocalPort is the single local port component.
func SingleLocalPort(port uint16) TFTComponent {
	return TFTComponent{Type: TFTSingleLocalPort, Value: binary.BigEndian.AppendUint16(nil, port)}
}

// LocalPortRange is the local port range component.
func LocalPortRange(low, high uint16) TFTComponent {
	return TFTComponent{Type: TFTLocalPortRange, Value: binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, low), high)}
}

// SingleRemotePort is the single remote port component.
func SingleRemotePort(port uint16) TFTComponent {
	return TFTComponent{Type: TFTSingleRemotePort, Value: binary.BigEndian.AppendUint16(nil, port)}
}

// RemotePortRange is the remote port range component.
func RemotePortRange(low, high uint16) TFTComponent {
	return TFTComponent{Type: TFTRemotePortRange, Value: binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, low), high)}
}

// AppendBinary appends the TFT value part to b.
func (t TrafficFlowTemplate) AppendBinary(b []byte) ([]byte, error) {
	if err := t.check(); err != nil {
		return b, err
	}

	w := nas.NewWriter(b)
	start := w.Len()

	count := len(t.Filters)
	if t.Operation == TFTDeleteFilters {
		count = len(t.DeleteIdentifiers)
	}

	e := uint8(0)
	if len(t.Parameters) > 0 {
		e = 1
	}

	w.U8(uint8(t.Operation)<<5 | e<<4 | uint8(count))

	if t.Operation == TFTDeleteFilters {
		for _, id := range t.DeleteIdentifiers {
			w.U8(id & 0x0F)
		}
	}

	for _, f := range t.Filters {
		w.U8(uint8(f.Direction)<<4 | f.Identifier&0x0F)
		w.U8(f.Precedence)
		w.LVFunc(func(c *nas.Writer) {
			for _, comp := range f.Components {
				c.U8(uint8(comp.Type))
				c.Raw(comp.Value)
			}
		})
	}

	for _, p := range t.Parameters {
		w.U8(p.Identifier)
		w.LV(p.Contents)
	}

	if w.Len()-start > maxTFTLen {
		return b, fmt.Errorf("nas/eps: TFT is %d octets, at most %d fit", w.Len()-start, maxTFTLen)
	}

	return w.Result(b)
}

// MarshalBinary encodes the TFT value part.
func (t TrafficFlowTemplate) MarshalBinary() ([]byte, error) { return t.AppendBinary(nil) }

func (t TrafficFlowTemplate) check() error {
	if t.Operation > tftOperationMax {
		return fmt.Errorf("nas/eps: TFT operation %d is reserved", t.Operation)
	}

	switch t.Operation {
	case TFTCreate, TFTAddFilters, TFTReplaceFilters:
		if len(t.Filters) == 0 || len(t.Filters) > maxTFTPacketFilter || len(t.DeleteIdentifiers) > 0 {
			return fmt.Errorf("nas/eps: TFT operation %d needs 1 to %d packet filters, has %d", t.Operation, maxTFTPacketFilter, len(t.Filters))
		}
	case TFTDeleteFilters:
		if len(t.DeleteIdentifiers) == 0 || len(t.DeleteIdentifiers) > maxTFTPacketFilter || len(t.Filters) > 0 {
			return fmt.Errorf("nas/eps: TFT packet filter deletion needs 1 to %d identifiers, has %d", maxTFTPacketFilter, len(t.DeleteIdentifiers))
		}
	default:
		if len(t.Filters) > 0 || len(t.DeleteIdentifiers) > 0 {
			return fmt.Errorf("nas/eps: TFT operation %d carries no packet filter", t.Operation)
		}
	}

	for _, f := range t.Filters {
		if f.Identifier > maxTFTPacketFilter || f.Direction > TFTBidirectional || len(f.Components) == 0 {
			return fmt.Errorf("nas/eps: TFT packet filter %d is malformed", f.Identifier)
		}

		seen := map[TFTComponentType]bool{}

		for _, c := range f.Components {
			if want, ok := tftComponentLen[c.Type]; !ok || want != len(c.Value) {
				return fmt.Errorf("nas/eps: TFT packet filter %d has a malformed component %#x", f.Identifier, uint8(c.Type))
			}

			if seen[c.Type] {
				return fmt.Errorf("nas/eps: TFT packet filter %d repeats component %#x", f.Identifier, uint8(c.Type))
			}

			seen[c.Type] = true
		}
	}

	return nil
}

// ParseTrafficFlowTemplate decodes a TFT value part.
func ParseTrafficFlowTemplate(b []byte) (TrafficFlowTemplate, error) {
	r := nas.NewReader(b)

	head, err := r.U8()
	if err != nil {
		return TrafficFlowTemplate{}, err
	}

	t := TrafficFlowTemplate{Operation: TFTOperation(head >> 5)}
	count := int(head & 0x0F)
	hasParameters := head&0x10 != 0

	if t.Operation == TFTDeleteFilters {
		for range count {
			id, err := r.U8()
			if err != nil {
				return TrafficFlowTemplate{}, err
			}

			t.DeleteIdentifiers = append(t.DeleteIdentifiers, id&0x0F)
		}
	} else if t.Operation != TFTDeleteExisting && t.Operation != TFTNoOperation && t.Operation != TFTIgnore {
		for range count {
			f, err := parseTFTPacketFilter(r)
			if err != nil {
				return TrafficFlowTemplate{}, err
			}

			t.Filters = append(t.Filters, f)
		}
	}

	for hasParameters && r.Remaining() > 0 {
		id, err := r.U8()
		if err != nil {
			return TrafficFlowTemplate{}, err
		}

		contents, err := r.LV()
		if err != nil {
			return TrafficFlowTemplate{}, err
		}

		t.Parameters = append(t.Parameters, TFTParameter{Identifier: id, Contents: contents})
	}

	if r.Remaining() > 0 {
		return TrafficFlowTemplate{}, fmt.Errorf("nas/eps: TFT has %d trailing octets", r.Remaining())
	}

	if err := t.check(); err != nil {
		return TrafficFlowTemplate{}, err
	}

	return t, nil
}

func parseTFTPacketFilter(r *nas.Reader) (TFTPacketFilter, error) {
	head, err := r.U8()
	if err != nil {
		return TFTPacketFilter{}, err
	}

	precedence, err := r.U8()
	if err != nil {
		return TFTPacketFilter{}, err
	}

	contents, err := r.LV()
	if err != nil {
		return TFTPacketFilter{}, err
	}

	f := TFTPacketFilter{Identifier: head & 0x0F, Direction: TFTDirection(head >> 4 & 0x03), Precedence: precedence}
	c := nas.NewReader(contents)

	for c.Remaining() > 0 {
		typ, err := c.U8()
		if err != nil {
			return TFTPacketFilter{}, err
		}

		n, ok := tftComponentLen[TFTComponentType(typ)]
		if !ok {
			return TFTPacketFilter{}, fmt.Errorf("nas/eps: TFT packet filter %d has an unsupported component %#x", f.Identifier, typ)
		}

		value, err := c.Bytes(n)
		if err != nil {
			return TFTPacketFilter{}, err
		}

		f.Components = append(f.Components, TFTComponent{Type: TFTComponentType(typ), Value: value})
	}

	return f, nil
}
