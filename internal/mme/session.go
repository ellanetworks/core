// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
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

// SessionTransferred drops the PDN connection the UE moved to 5GS, freeing its
// EPS bearer identity and leaving the session anchored (TS 23.502 §4.11.2.3
// step 10).
func (m *MME) SessionTransferred(ctx context.Context, imsi string, ebi uint8) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	m.DropPDN(ue, ebi)

	logger.From(ctx, logger.MmeLog).Info("dropped PDN connection moved to 5GS",
		zap.String("imsi", imsi), zap.Uint8("ebi", ebi))

	// An attached EPS UE always holds at least one PDN connection
	// (TS 23.401 §5.10.3), so moving its last one to 5GS detaches it.
	if ue.PDNCount() == 0 {
		logger.From(ctx, logger.MmeLog).Info("last PDN connection moved to 5GS, detaching",
			zap.String("imsi", imsi))
		ue.TransitionTo(EMMDeregistered)
		m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)

		return
	}

	// No NAS PDU: TS 23.502 §4.11.2.3 step 10 excepts the deactivation toward the
	// UE.
	if ue.Connected() {
		m.sendERABRelease(ctx, ue, &PdnConnection{Ebi: ebi}, nil)
	}
}
