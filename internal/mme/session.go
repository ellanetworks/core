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

// SessionTransferred drops the PDN connection for a session the UE moved to
// 5GS, without releasing the anchor: the session, its user plane and the UE
// address all survive on the other access (TS 23.502 §4.11.2.3 step 10). The
// MME's own release path would otherwise tear down a session the UE is using
// over N3.
//
// ref names the exact session instance, so a report arriving after the UE
// re-established a PDN connection under the same bearer identity is ignored
// rather than dropping the live one.
//
// §4.11.2.3 step 10 runs the EPS bearer deactivation of TS 23.401 §5.4.4.1
// except its steps 4-7 — the NAS exchange with the UE, which has left EPS — so
// the S1 E-RAB release is still in scope: a UE in dual-registration mode stays
// on E-UTRAN while it moves its sessions one at a time, and the moved bearer's
// radio resources would otherwise leak at the eNB. An attached UE always holds
// at least one PDN connection (TS 23.401 §5.10.3), so losing its last one
// detaches it; leaving it attached with none would hard-fail its next Initial
// Context Setup and Attach Accept.
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
		// The detach takes the whole S1 context, including this bearer's E-RAB, so
		// no separate release is needed.
		ue.TransitionTo(EMMDeregistered)
		m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)

		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	// No NAS PDU: the UE is not being told its bearer was deactivated — it moved
	// the session itself and §4.11.2.3 step 10 excludes the NAS exchange — only the
	// eNB that this E-RAB is over.
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

// takePDNByRef removes the PDN connection for ebi only when it still names ref,
// and reports whether it was the UE's last one. Both under the lock, so the
// caller's detach decision cannot race a concurrent release.
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

// UnwindPDN undoes a PDN connection the MME set up but could not deliver to the
// UE. A connection established with request type "handover" moved a session the
// UE holds on the other access, and that session must survive: releasing it
// would tear down one the UE is still using. Anything else is released as usual.
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
