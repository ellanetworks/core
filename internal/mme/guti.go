// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

// releaseMTMSIsLocked unindexes and frees both the UE's current M-TMSI and any
// pending old one from an in-flight GUTI reallocation. The caller holds m.mu.
func (m *MME) releaseMTMSIsLocked(ue *UeContext) {
	if ue.tmsi.Uint32() != 0 {
		delete(m.uesByTmsi, ue.tmsi)
		m.freeMTMSILocked(ue.tmsi)
	}

	if ue.oldTmsi.Uint32() != 0 {
		delete(m.uesByTmsi, ue.oldTmsi)
		m.freeMTMSILocked(ue.oldTmsi)
	}
}

// freeMTMSILocked returns an M-TMSI to the allocator for reuse. The caller holds
// m.mu.
func (m *MME) freeMTMSILocked(t etsi.TMSI) {
	m.tmsi.Free(t)
}

// ReallocateGUTI stages a new GUTI without dropping the old one: both M-TMSIs
// stay indexed (and the UE keeps being paged with the old one) until the UE
// acknowledges — ATTACH COMPLETE or TRACKING AREA UPDATE COMPLETE — which
// commits the new GUTI (TS 24.301 §5.5.1.2.7, §5.5.3.2.4: the old GUTI stays
// valid until completion). A reallocation already in flight (e.g. on a
// retransmitted attach or TAU) reuses the staged M-TMSI.
func (m *MME) ReallocateGUTI(ctx context.Context, ue *UeContext, plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (eps.EPSMobileIdentity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.oldTmsi.Uint32() == 0 {
		tmsi, err := m.tmsi.Allocate(ctx)
		if err != nil {
			return eps.EPSMobileIdentity{}, fmt.Errorf("allocate M-TMSI: %w", err)
		}

		ue.oldTmsi = ue.tmsi
		ue.tmsi = tmsi
		m.uesByTmsi[ue.tmsi] = ue
	}

	return eps.EPSMobileIdentity{
		Type:       eps.IdentityGUTI,
		MCC:        plmn.Mcc,
		MNC:        plmn.Mnc,
		MMEGroupID: mmeGroupID,
		MMECode:    mmeCode,
		MTMSI:      ue.tmsi.Uint32(),
	}, nil
}

// CommitGUTIRealloc finalises a GUTI reallocation once the UE acknowledges it:
// the old M-TMSI is unindexed and freed, leaving only the new GUTI valid.
func (m *MME) CommitGUTIRealloc(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.oldTmsi.Uint32() != 0 && ue.oldTmsi != ue.tmsi {
		delete(m.uesByTmsi, ue.oldTmsi)
		m.freeMTMSILocked(ue.oldTmsi)
	}

	ue.oldTmsi = etsi.TMSI{}
}
