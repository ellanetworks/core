// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

func (m *Manager) PartnerName(partnerID string) string {
	snap := m.snap.Load()
	if snap == nil {
		return ""
	}

	if p, ok := snap.partners[partnerID]; ok {
		return p.name
	}

	return ""
}
