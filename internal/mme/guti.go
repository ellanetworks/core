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

func (m *MME) SendGUTIReallocationCommand(ctx context.Context, ue *UeContext) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: get operator PLMN", zap.Error(err))
		return
	}

	mmeGroupID, mmeCode, err := m.MmeIdentity(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: get GUMMEI", zap.Error(err))
		return
	}

	guti, err := m.ReallocateGUTI(ctx, ue, plmn, mmeGroupID, mmeCode)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: allocate GUTI", zap.Error(err))
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.MmeLog).Warn("GUTI reallocation: UE released before the command could be sent")
		return
	}

	plain, err := (&eps.GUTIReallocationCommand{GUTI: guti}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("GUTI reallocation: build the command", zap.Error(err))
		return
	}

	if err := ueConn.SendProtectedNASTransport(ctx, plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		ReportProtectFailure(ctx, ueConn, "GUTI Reallocation Command", err)

		return
	}

	ueConn.ArmNASGuardAbortOnly("GUTI Reallocation Command", plain, eps.SHTIntegrityProtectedCiphered, func() {
		logger.From(ctx, logger.MmeLog).Warn("GUTI reallocation aborted: no GUTI Reallocation Complete after T3450 retransmissions",
			zap.String("imsi", ue.IMSI()))
	})
}

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

func (m *MME) freeMTMSILocked(t etsi.TMSI) {
	m.tmsi.Free(t)
}

func (m *MME) ReallocateGUTI(ctx context.Context, ue *UeContext, plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (eps.EPSMobileIdentity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.oldTmsi == etsi.InvalidTMSI {
		tmsi, err := m.tmsi.Allocate(ctx)
		if err != nil {
			return eps.EPSMobileIdentity{}, fmt.Errorf("allocate M-TMSI: %w", err)
		}

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
