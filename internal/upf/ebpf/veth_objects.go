// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"net/netip"
)

func (bpfObjects *BpfObjects) PutTunnel(dstIPv6 netip.Addr, uePrefix netip.Prefix) error {
	key := N3N6EntrypointIn6Addr{}
	key.In6U.U6Addr8 = dstIPv6.As16()

	val := N3N6EntrypointVethTunnelInfo{}
	val.UePrefix.In6U.U6Addr8 = uePrefix.Masked().Addr().As16()

	return bpfObjects.VethTunnels.Put(&key, &val)
}

func (bpfObjects *BpfObjects) DeleteTunnel(dstIPv6 netip.Addr) error {
	key := N3N6EntrypointIn6Addr{}
	key.In6U.U6Addr8 = dstIPv6.As16()

	return bpfObjects.VethTunnels.Delete(&key)
}
