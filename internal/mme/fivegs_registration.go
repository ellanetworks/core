// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
)

func (m *MME) SupersedeFiveGSRegistration(ctx context.Context, ue *UeContext) {
	if m.FiveGS == nil || ue == nil {
		return
	}

	supi := ue.Supi()
	if !supi.IsIMSI() {
		return
	}

	if ue.RetainsFiveGSRegistration {
		return
	}

	if _, relocating := m.RelocationToFiveGS(ue); relocating {
		return
	}

	if ue.IdleMobilityTo5GSPending() {
		return
	}

	m.FiveGS.CancelRegistration(ctx, supi)
}
