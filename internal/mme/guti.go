// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// SendGUTIReallocationCommand reassigns the UE's GUTI with the standalone GUTI
// reallocation procedure: it stages a new GUTI, sends a protected GUTI REALLOCATION
// COMMAND, and arms T3450 so the command is retransmitted until the UE acknowledges
// with GUTI REALLOCATION COMPLETE (TS 24.301 §5.4.1). It mirrors the AMF's
// post-Service-Request CONFIGURATION UPDATE, giving a fresh temporary identity when
// the UE returns from idle with no accept message to carry one. The UE keeps using
// the old GUTI until it acknowledges (§5.4.1.4).
func (m *MME) SendGUTIReallocationCommand(ctx context.Context, ue *UeContext) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: get operator PLMN", zap.Error(err))
		return
	}

	mmeGroupID, mmeCode := m.MmeIdentity()

	guti, err := m.ReallocateGUTI(ctx, ue, plmn, mmeGroupID, mmeCode)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: allocate GUTI", zap.Error(err))
		return
	}

	wire, err := ue.ProtectDownlinkMessage(&eps.GUTIReallocationCommand{GUTI: guti})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: protect command", zap.Error(err))
		return
	}

	// On T3450 exhaustion the reallocation is aborted, leaving the UE connected with
	// both the old and new GUTI valid (TS 24.301 §5.4.1.6 a) — a later Service Request
	// re-initiates with the staged M-TMSI. Abort-only, not a release: it mirrors the
	// AMF's CONFIGURATION UPDATE (T3555), which keeps the UE on exhaustion.
	ue.Conn().ArmNASGuardAbortOnly("GUTI Reallocation Command", wire, func() {
		logger.From(ctx, logger.MmeLog).Warn("GUTI reallocation aborted: no GUTI Reallocation Complete after T3450 retransmissions",
			zap.String("imsi", ue.IMSI()))
	})
	ue.Conn().SendDownlinkNASTransport(ctx, wire)
}

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
