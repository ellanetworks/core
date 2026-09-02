// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"

	naslib "github.com/ellanetworks/core/nas"
)

// ProtocolConfigurationOptions is the (extended) protocol configuration options
// element (TS 24.008 §10.5.6.3). The element predates 5GS and both stacks carry
// it, so one rendering serves them.
//
// Containers are listed rather than flattened into named flags: an identifier
// means different things in each direction — 000DH is a DNS server address
// request uplink and the address itself downlink — and the contents belong to
// the protocol the identifier names, so they are kept as bytes.
type ProtocolConfigurationOptions struct {
	ConfigProtocol uint8          `json:"config_protocol"`
	Direction      string         `json:"direction,omitempty"`
	Containers     []PCOContainer `json:"containers,omitempty"`
	Error          string         `json:"error,omitempty"`
}

type PCOContainer struct {
	ID   uint16 `json:"id"`
	Name string `json:"name,omitempty"`
	// Value is the contents read as what the identifier names them, for the
	// containers TS 24.008 §10.5.6.3 gives a fixed width and meaning.
	Value string `json:"value,omitempty"`
	// Hex is the contents of a container this decoder does not read: one whose
	// payload belongs to another protocol, or is a whole element of another
	// specification, or did not have the width the identifier calls for. A
	// container that was read carries its value instead, the way every other
	// decoded value here does.
	Hex string `json:"hex,omitempty"`
}

// containerValue reads the contents of the containers the spec gives a fixed
// width and meaning. Reading is direction-scoped: uplink an identifier is an
// empty request, downlink it carries the value being requested.
func containerValue(id uint16, dir naslib.PCODirection, b []byte) string {
	if dir != naslib.PCONetworkToMS || len(b) == 0 {
		return ""
	}

	switch id {
	// one IPv4 address (RFC 791)
	case 0x0009, 0x000c, 0x000d:
		if len(b) == net.IPv4len {
			return net.IP(b).String()
		}
	// one IPv6 address, encoded as 128 bits (RFC 4291)
	case 0x0001, 0x0003, 0x0007:
		if len(b) == net.IPv6len {
			return net.IP(b).String()
		}
	// an IPv6 prefix: the address followed by its length in bits
	case 0x0008:
		if len(b) == net.IPv6len+1 {
			return fmt.Sprintf("%s/%d", net.IP(b[:net.IPv6len]), b[net.IPv6len])
		}
	// a link MTU in octets, most significant octet first
	case 0x0010, 0x0015:
		if len(b) == 2 {
			return strconv.Itoa(int(binary.BigEndian.Uint16(b)))
		}
	case 0x0005:
		return naslib.PCOSelectedBearerControlModeName(b[0])
	case 0x0014:
		return naslib.PCONBIFOMModeName(b[0])
	case 0x0017:
		return naslib.PCOPSDataOffStatusName(b[0])
	}

	return ""
}

func pcoDirectionName(d naslib.PCODirection) string {
	switch d {
	case naslib.PCOMSToNetwork:
		return "uplink"
	case naslib.PCONetworkToMS:
		return "downlink"
	default:
		return ""
	}
}

// ExtendedPCO renders the element, or nil when it is absent.
func ExtendedPCO(opts *naslib.ProtocolConfigurationOptions) *ProtocolConfigurationOptions {
	if opts == nil {
		return nil
	}

	out := &ProtocolConfigurationOptions{
		ConfigProtocol: opts.ConfigProtocol,
		Direction:      pcoDirectionName(opts.Direction),
	}

	for _, c := range opts.Containers {
		container := PCOContainer{
			ID:    c.ID,
			Name:  naslib.PCOContainerName(c.ID, opts.Direction),
			Value: containerValue(c.ID, opts.Direction, c.Content),
		}

		if container.Value == "" {
			container.Hex = hex.EncodeToString(c.Content)
		}

		out.Containers = append(out.Containers, container)
	}

	return out
}
