// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// detachTypeReattachNotRequired is the network-originating detach type meaning
// the UE shall not re-attach (TS 24.301).
const detachTypeReattachNotRequired uint8 = 2

// DetachSubscriber sends a network-initiated DETACH REQUEST (TS 24.301)
// to the attached UE for imsi, if any, when a subscriber is deleted. The
// request is guarded by T3422: if the UE does not reply with Detach Accept it
// is retransmitted, and on exhaustion the UE context is released regardless, so
// a silent UE cannot leak the context.
func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	// An idle UE (ECM-IDLE) holds no S1 connection to carry the DETACH REQUEST, so
	// release its sessions and context locally. The deleted subscriber is denied at
	// its next contact, as it can no longer authenticate.
	if !m.UeConnected(ue) {
		ue.SetEMMState(EMMDeregistered)
		logger.MmeLog.Info("releasing idle UE on subscriber deletion", zap.String("imsi", imsi))
		m.ReleaseAllSessions(ue)
		m.RemoveUe(ue)

		return
	}

	// The connected UE is asked to detach; T3422 guards the DETACH REQUEST and the
	// UE stays EMM-DEREGISTERED-INITIATED until it accepts or the guard exhausts.
	ue.SetEMMState(EMMDeregistrationInitiated)

	logger.MmeLog.Info("network-initiated detach (subscriber deleted)",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", imsi))

	naspdu, err := m.ProtectDownlinkMessage(ue, &eps.DetachRequestNetwork{TypeOfDetach: detachTypeReattachNotRequired})
	if err != nil {
		logger.MmeLog.Error("failed to protect Detach Request", zap.Error(err))
		return
	}

	m.SendDownlink(ctx, ue, naspdu)
	m.ArmNASGuard(ue, "Detach Request", naspdu)
}
