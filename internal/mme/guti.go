// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// SendGUTIReallocationCommand runs the standalone GUTI reallocation procedure, arming
// T3450 to retransmit the GUTI REALLOCATION COMMAND until the UE acknowledges; the UE
// keeps the old GUTI until then (TS 24.301 §5.4.1, §5.4.1.4).
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

	// Snapshotted, not re-read: this runs off the dispatch goroutine and the
	// unlocked calls below let a concurrent release nil ue.Conn().
	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.MmeLog).Warn("GUTI reallocation: UE released before the command could be sent")
		return
	}

	wire, err := ue.ProtectDownlinkMessage(&eps.GUTIReallocationCommand{GUTI: guti})
	if err != nil {
		ReportProtectFailure(ctx, ueConn, "GUTI Reallocation Command", err)
		return
	}

	// On T3450 exhaustion the reallocation is abort-only, not a UE release: the UE stays
	// connected with both old and new GUTI valid, and a later Service Request re-initiates
	// with the staged M-TMSI (TS 24.301 §5.4.1.6 a).
	ueConn.ArmNASGuardAbortOnly("GUTI Reallocation Command", wire, func() {
		logger.From(ctx, logger.MmeLog).Warn("GUTI reallocation aborted: no GUTI Reallocation Complete after T3450 retransmissions",
			zap.String("imsi", ue.IMSI()))
	})
	ueConn.SendDownlinkNASTransport(ctx, wire)
}

// releaseMTMSIsLocked unindexes and frees both the UE's current M-TMSI and any
// pending old one from an in-flight GUTI reallocation. The caller holds m.mu.
func (m *MME) releaseMTMSIsLocked(ue *UeContext) {
	if ue.tmsi != etsi.InvalidTMSI {
		delete(m.uesByTmsi, ue.tmsi)
		m.freeMTMSILocked(ue.tmsi)
	}

	if ue.oldTmsi != etsi.InvalidTMSI {
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

	if ue.oldTmsi == etsi.InvalidTMSI {
		tmsi, err := m.tmsi.Allocate(ctx)
		if err != nil {
			return eps.EPSMobileIdentity{}, fmt.Errorf("allocate M-TMSI: %w", err)
		}

		// Also read through Tmsi()/OldTmsi() by callers holding no registry lock,
		// so writes take ue.mu too. Order m.mu → ue.mu.
		ue.mu.Lock()
		ue.oldTmsi = ue.tmsi
		ue.tmsi = tmsi
		ue.mu.Unlock()

		m.uesByTmsi[ue.tmsi] = ue
	}

	return eps.GUTIIdentity(eps.GUTI{
		PLMN:       nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc},
		MMEGroupID: mmeGroupID,
		MMECode:    mmeCode,
		TMSI:       tmsiOctets(ue.tmsi),
	}), nil
}

// CommitGUTIRealloc finalises a GUTI reallocation once the UE acknowledges it:
// the old M-TMSI is unindexed and freed, leaving only the new GUTI valid.
func (m *MME) CommitGUTIRealloc(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.oldTmsi != etsi.InvalidTMSI && ue.oldTmsi != ue.tmsi {
		delete(m.uesByTmsi, ue.oldTmsi)
		m.freeMTMSILocked(ue.oldTmsi)
	}

	ue.mu.Lock()
	ue.oldTmsi = etsi.InvalidTMSI
	ue.mu.Unlock()
}

// tmsiOctets is the allocator's TMSI in the four octets a GUTI carries.
func tmsiOctets(t etsi.TMSI) [4]byte {
	var out [4]byte

	binary.BigEndian.PutUint32(out[:], t.Uint32())

	return out
}
