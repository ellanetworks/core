// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"net"

	"github.com/ellanetworks/core/nas/common"
)

// Protocol/container identifiers for the (extended) protocol configuration
// options (TS 24.008 §10.5.6.3). A request (uplink) and its answer (downlink)
// share the container identifier.
const (
	PCOContainerDNSServerIPv6Address uint16 = 0x0003
	PCOContainerDNSServerIPv4Address uint16 = 0x000D
	PCOContainerIPv4LinkMTU          uint16 = 0x0010
)

// PCO builds the protocol configuration options IE content (TS 24.501
// §9.11.4.6): a configuration-protocol octet followed by protocol/container
// units, each an identifier, a length, and its contents.
type PCO struct {
	units []pcoUnit
}

type pcoUnit struct {
	id       uint16
	contents []byte
}

// AddDNSServerIPv4Address appends a downlink DNS server IPv4 address unit.
func (p *PCO) AddDNSServerIPv4Address(dns net.IP) {
	p.units = append(p.units, pcoUnit{id: PCOContainerDNSServerIPv4Address, contents: dns.To4()})
}

// AddDNSServerIPv6Address appends a downlink DNS server IPv6 address unit.
func (p *PCO) AddDNSServerIPv6Address(dns net.IP) {
	p.units = append(p.units, pcoUnit{id: PCOContainerDNSServerIPv6Address, contents: dns.To16()})
}

// AddIPv4LinkMTU appends a downlink IPv4 link MTU unit.
func (p *PCO) AddIPv4LinkMTU(mtu uint16) {
	p.units = append(p.units, pcoUnit{id: PCOContainerIPv4LinkMTU, contents: []byte{uint8(mtu >> 8), uint8(mtu)}})
}

// Empty reports whether no protocol/container units have been added.
func (p *PCO) Empty() bool { return len(p.units) == 0 }

// ParsePCOContainerIDs reads a PCO IE content and returns the protocol/container
// identifiers it carries, in order (TS 24.008 §10.5.6.3). It is used to inspect
// a UE's requested containers; the container contents are not returned.
func ParsePCOContainerIDs(content []byte) ([]uint16, error) {
	r := common.NewReader(content)

	if _, err := r.U8(); err != nil { // configuration protocol octet
		return nil, err
	}

	var ids []uint16

	for r.Remaining() > 0 {
		id, err := r.U16()
		if err != nil {
			return nil, err
		}

		length, err := r.U8()
		if err != nil {
			return nil, err
		}

		if _, err := r.Bytes(int(length)); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

// Marshal encodes the PCO IE content. The first octet has the extension bit set
// and configuration protocol 0 (TS 24.008 §10.5.6.3).
func (p *PCO) Marshal() []byte {
	var w common.Writer

	w.U8(0x80) // extension bit set, configuration protocol = 0

	for _, u := range p.units {
		w.U16(u.id)
		w.U8(uint8(len(u.contents)))
		w.Raw(u.contents)
	}

	return w.Bytes()
}
