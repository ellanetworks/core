// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandleERABSetupResponse processes the eNB's answer to an E-RAB SETUP REQUEST
// (TS 36.413 §8.2.1): it records the eNB S1-U endpoint of each established E-RAB
// on the anchor session, and releases any E-RAB the eNB failed to set up.
func HandleERABSetupResponse(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseERABSetupResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Setup Response", zap.Error(err))
		return
	}

	ue, ueConn, ok := resolveUEIDs(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcERABSetup, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), msg.Diagnostics())

	ue.TouchLastSeen()
	captureUserLocation(ueConn, msg.UserLocationInformation)

	result := m.ReconcileBearersToRAN(ctx, ue, mme.RANBearers{
		Present:  setupBearers(ctx, ueConn.MMEUES1APID, bearerSetupBearers(msg.ERABSetup)),
		Rejected: failedERABIDs(msg.ERABFailedToSetup),
	})

	logger.MmeLog.Info("additional PDN connection radio legs reconciled",
		zap.String("imsi", ue.IMSI()),
		zap.Int("e-rabs-setup", len(result.Applied)),
		zap.Int("e-rabs-released", len(result.Released)))
}

// bearerSetupBearers projects an E-RAB SETUP RESPONSE setup list.
func bearerSetupBearers(items []s1ap.ERABSetupItemBearerSURes) []setupBearer {
	out := make([]setupBearer, 0, len(items))
	for _, e := range items {
		out = append(out, setupBearer{ERABID: e.ERABID, TransportLayerAddress: e.TransportLayerAddress, GTPTEID: e.GTPTEID})
	}

	return out
}
