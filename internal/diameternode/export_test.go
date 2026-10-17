// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import "net/netip"

func (m *Manager) SetLocalAddrForTest(f func(remote netip.Addr) (netip.Addr, error)) {
	m.localAddr = f
}
