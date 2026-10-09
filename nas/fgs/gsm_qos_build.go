// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// RemoteAddressComponent is the IPv4 or IPv6 remote address component (TS 24.501 table 9.11.4.13.1).
func RemoteAddressComponent(p netip.Prefix) PacketFilterComponent {
	p = p.Masked()

	if p.Addr().Is4() {
		mask := netip.PrefixFrom(netip.AddrFrom4([4]byte{0xFF, 0xFF, 0xFF, 0xFF}), p.Bits()).Masked().Addr().As4()
		addr := p.Addr().As4()

		return PacketFilterComponent{Type: pfComponentTypeIPv4RemoteAddress, Value: append(addr[:], mask[:]...)}
	}

	addr := p.Addr().As16()

	return PacketFilterComponent{Type: pfComponentTypeIPv6RemoteAddress, Value: append(addr[:], uint8(p.Bits()))}
}

// ProtocolComponent is the protocol identifier/next header component.
func ProtocolComponent(proto uint8) PacketFilterComponent {
	return PacketFilterComponent{Type: pfComponentTypeProtocolIdentifier, Value: []byte{proto}}
}

// SingleLocalPortComponent is the single local port component.
func SingleLocalPortComponent(port uint16) PacketFilterComponent {
	return PacketFilterComponent{Type: pfComponentTypeSingleLocalPort, Value: binary.BigEndian.AppendUint16(nil, port)}
}

// SingleRemotePortComponent is the single remote port component.
func SingleRemotePortComponent(port uint16) PacketFilterComponent {
	return PacketFilterComponent{Type: pfComponentTypeSingleRemotePort, Value: binary.BigEndian.AppendUint16(nil, port)}
}

// BitRateQoSFlowParameter encodes a GFBR or MFBR parameter in the smallest unit that holds the rate, rounded up (TS 24.501 §9.11.4.12).
func BitRateQoSFlowParameter(id QoSFlowParameterID, bps uint64) (QoSFlowParameter, error) {
	switch id {
	case QoSFlowParamGFBRUplink, QoSFlowParamGFBRDownlink, QoSFlowParamMFBRUplink, QoSFlowParamMFBRDownlink:
	default:
		return QoSFlowParameter{}, fmt.Errorf("nas/fgs: %s carries no bit rate", id)
	}

	kbps := (bps + 999) / 1000

	for unit := qosRateUnit1Kbps; unit <= qosRateUnitMax; unit++ {
		step := qosRateUnitKbps(unit)

		v := (kbps + step - 1) / step
		if v <= 0xFFFF {
			return QoSFlowParameter{ID: id, Value: []byte{unit, byte(v >> 8), byte(v)}}, nil
		}
	}

	return QoSFlowParameter{}, fmt.Errorf("nas/fgs: %d bps exceeds the largest QoS flow bit rate", bps)
}
