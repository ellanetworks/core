// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// takeAllPDNs detaches and returns every PDN connection from the UE under the
// lock, so the caller can release the sessions without holding it.
func takeAllPDNs(ue *UeContext) []*PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]*PdnConnection, 0, len(ue.Pdns))
	for _, p := range ue.Pdns {
		out = append(out, p)
	}

	ue.Pdns = nil
	ue.DefaultEBI = 0

	return out
}

// SnapshotPDNs returns the UE's PDN connections as a slice taken under the lock,
// so the reconciler does not iterate the map while a NAS handler mutates it.
func (m *MME) SnapshotPDNs(ue *UeContext) []*PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]*PdnConnection, 0, len(ue.Pdns))
	for _, p := range ue.Pdns {
		out = append(out, p)
	}

	return out
}

// ReleasePDN tears down a PDN connection's anchor session and removes it from the
// UE, freeing its EPS bearer identity.
func (m *MME) ReleasePDN(ctx context.Context, ue *UeContext, p *PdnConnection) {
	if err := m.Session.ReleaseEPSSession(ctx, p.SessionRef); err != nil {
		logger.MmeLog.Warn("failed to release PDN connection session",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Error(err))
	}

	ue.mu.Lock()
	delete(ue.Pdns, p.Ebi)

	if ue.DefaultEBI == p.Ebi {
		ue.DefaultEBI = 0
	}

	ue.mu.Unlock()
}

// DeactivatePDN completes p's teardown in the MME: an additional PDN or a
// disconnect releases only that connection and leaves the UE connected;
// deactivating the default bearer releases the UE context so the UE re-attaches
// (TS 24.301 §6.4.4).
func (m *MME) DeactivatePDN(ctx context.Context, ue *UeContext, p *PdnConnection) {
	if ue.BearerReleaseOnly(p) {
		m.ReleasePDN(ctx, ue, p)
		return
	}

	ue.TransitionTo(EMMDeregistered)
	m.ReleaseAllSessions(ctx, ue)
	m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)
}

// ReleaseAllSessions releases every PDN connection's anchor session and clears
// them from the UE.
func (m *MME) ReleaseAllSessions(ctx context.Context, ue *UeContext) {
	for _, p := range takeAllPDNs(ue) {
		if err := m.Session.ReleaseEPSSession(ctx, p.SessionRef); err != nil {
			logger.MmeLog.Warn("failed to release PDN connection session",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Error(err))
		}
	}
}

// DeactivateAllSessions buffers every PDN connection's downlink so data for the
// idle UE triggers paging (TS 23.401), without releasing the sessions.
func (m *MME) DeactivateAllSessions(ctx context.Context, ue *UeContext) {
	for _, p := range m.SnapshotPDNs(ue) {
		if err := m.Session.DeactivateEPSSession(ctx, p.SessionRef); err != nil {
			logger.MmeLog.Warn("failed to deactivate PDN connection session for paging",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Error(err))
		}
	}
}

func (m *MME) SessionTransferred(ctx context.Context, imsi string, ebi uint8, ref string) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	p, last := takePDNByRef(ue, ebi, ref)
	if p == nil {
		logger.From(ctx, logger.MmeLog).Debug("ignoring a transfer report for a PDN connection this MME no longer holds",
			zap.String("imsi", imsi), zap.Uint8("ebi", ebi), zap.String("ref", ref))

		return
	}

	logger.From(ctx, logger.MmeLog).Info("PDN connection moved to 5GS; dropping the EPS routing context",
		zap.String("imsi", imsi), zap.Uint8("ebi", ebi), zap.Bool("last-pdn", last))

	if last {
		ue.TransitionTo(EMMDeregistered)
		m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)

		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	cmd := &s1ap.ERABReleaseCommand{
		ERABToBeReleased: []s1ap.ERABItem{{
			ERABID: s1ap.ERABID(ebi),
			Cause:  CauseNASNormalRelease,
		}},
	}

	if err := conn.SendERABRelease(ctx, cmd); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to release the E-RAB of a moved PDN connection",
			zap.String("imsi", imsi), zap.Uint8("ebi", ebi), zap.Error(err))
	}
}

func takePDNByRef(ue *UeContext, ebi uint8, ref string) (p *PdnConnection, last bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	p, ok := ue.Pdns[ebi]
	if !ok || p.SessionRef != ref {
		return nil, false
	}

	delete(ue.Pdns, ebi)

	if ue.DefaultEBI == ebi {
		ue.DefaultEBI = 0
	}

	return p, len(ue.Pdns) == 0
}

func (m *MME) UnwindPDN(ctx context.Context, ue *UeContext, p *PdnConnection, moved bool) {
	if !moved {
		m.ReleasePDN(ctx, ue, p)
		return
	}

	m.Session.AbandonEPSTransfer(ctx, p.SessionRef)

	ue.mu.Lock()
	delete(ue.Pdns, p.Ebi)

	if ue.DefaultEBI == p.Ebi {
		ue.DefaultEBI = 0
	}

	ue.mu.Unlock()
}
