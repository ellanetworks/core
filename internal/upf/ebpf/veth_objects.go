// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

// veth_tunnels programming; the map and its program live in bpf/n3n6_bpf.c.

import (
	"net/netip"
)

// VethTunnelInfo holds the Go-side representation of a veth tunnel map entry.
type VethTunnelInfo struct {
	TEID       uint32
	LocalAddr  netip.Addr // UPF N3 transport address (IPv4 or IPv6)
	RemoteAddr netip.Addr // eNB/gNB N3 transport address (IPv4 or IPv6)
	QFI        uint8
	S1U        bool // 4G S1-U: encapsulate PSC-less (no PDU Session Container)
}

// PutTunnel programs a tunnel entry in the veth_tunnels BPF map.
// The key is the inner IPv6 destination address.
func (bpfObjects *BpfObjects) PutTunnel(dstIPv6 netip.Addr, info VethTunnelInfo) error {
	key := N3N6EntrypointIn6Addr{}
	key.In6U.U6Addr8 = dstIPv6.As16()

	localAddr := N3N6EntrypointIn6Addr{}
	localAddr.In6U.U6Addr8 = IPToIn6Addr(info.LocalAddr)

	remoteAddr := N3N6EntrypointIn6Addr{}
	remoteAddr.In6U.U6Addr8 = IPToIn6Addr(info.RemoteAddr)

	val := N3N6EntrypointVethTunnelInfo{
		Teid:       info.TEID,
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Qfi:        info.QFI,
	}

	if info.S1U {
		val.NoPsc = 1
	}

	return bpfObjects.VethTunnels.Put(&key, &val)
}

// DeleteTunnel removes a tunnel entry from the veth_tunnels BPF map.
func (bpfObjects *BpfObjects) DeleteTunnel(dstIPv6 netip.Addr) error {
	key := N3N6EntrypointIn6Addr{}
	key.In6U.U6Addr8 = dstIPv6.As16()

	return bpfObjects.VethTunnels.Delete(&key)
}
