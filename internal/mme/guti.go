// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// assignGUTI allocates a fresh, unpredictable M-TMSI, builds the GUTI for the
// serving PLMN and the configured MME identity (group ID + code, TS 23.003),
// and indexes the UE by its M-TMSI for later S-TMSI-addressed procedures. The
// MME reallocates a GUTI on every IMSI attach (TS 24.301). Any M-TMSI it holds
// is freed for reuse.
func (m *MME) assignGUTI(ue *UeContext, plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) eps.EPSMobileIdentity {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.mtmsi != 0 {
		delete(m.byMTMSI, ue.mtmsi)
		m.freeMTMSILocked(ue.mtmsi)
	}

	tmsi, err := m.mtmsi.Allocate(context.Background())
	if err != nil {
		logger.MmeLog.Error("failed to allocate M-TMSI", zap.Error(err))
	}

	mtmsi := tmsi.Uint32()

	ue.mtmsi = mtmsi
	m.byMTMSI[mtmsi] = ue

	return eps.EPSMobileIdentity{
		Type:       eps.IdentityGUTI,
		MCC:        plmn.Mcc,
		MNC:        plmn.Mnc,
		MMEGroupID: mmeGroupID,
		MMECode:    mmeCode,
		MTMSI:      mtmsi,
	}
}

// releaseMTMSIsLocked unindexes and frees both the UE's current M-TMSI and any
// pending old one from an in-flight GUTI reallocation. The caller holds m.mu.
func (m *MME) releaseMTMSIsLocked(ue *UeContext) {
	if ue.mtmsi != 0 {
		delete(m.byMTMSI, ue.mtmsi)
		m.freeMTMSILocked(ue.mtmsi)
	}

	if ue.oldMTMSI != 0 {
		delete(m.byMTMSI, ue.oldMTMSI)
		m.freeMTMSILocked(ue.oldMTMSI)
	}
}

// freeMTMSILocked returns an M-TMSI to the allocator for reuse. The caller holds
// m.mu.
func (m *MME) freeMTMSILocked(mtmsi uint32) {
	t, err := etsi.NewTMSI(mtmsi)
	if err != nil {
		return
	}

	m.mtmsi.Free(t)
}

// reallocateGUTI stages a new GUTI for a TAU without dropping the old one: both
// M-TMSIs stay indexed (and the UE keeps being paged with the old one) until
// TRACKING AREA UPDATE COMPLETE commits the new GUTI (TS 24.301). A
// reallocation already in flight (e.g. on a retransmitted TAU) reuses the staged
// M-TMSI.
func (m *MME) reallocateGUTI(ue *UeContext, plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) eps.EPSMobileIdentity {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.oldMTMSI == 0 {
		tmsi, err := m.mtmsi.Allocate(context.Background())
		if err != nil {
			logger.MmeLog.Error("failed to allocate M-TMSI", zap.Error(err))
		}

		ue.oldMTMSI = ue.mtmsi
		ue.mtmsi = tmsi.Uint32()
		m.byMTMSI[ue.mtmsi] = ue
	}

	return eps.EPSMobileIdentity{
		Type:       eps.IdentityGUTI,
		MCC:        plmn.Mcc,
		MNC:        plmn.Mnc,
		MMEGroupID: mmeGroupID,
		MMECode:    mmeCode,
		MTMSI:      ue.mtmsi,
	}
}

// commitGUTIRealloc finalises a GUTI reallocation once the UE acknowledges it:
// the old M-TMSI is unindexed and freed, leaving only the new GUTI valid.
func (m *MME) commitGUTIRealloc(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.oldMTMSI != 0 && ue.oldMTMSI != ue.mtmsi {
		delete(m.byMTMSI, ue.oldMTMSI)
		m.freeMTMSILocked(ue.oldMTMSI)
	}

	ue.oldMTMSI = 0
}
