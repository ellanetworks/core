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

// handleUECapabilityInfoIndication stores the UE Radio Capability reported by the
// eNB (TS 36.413), replayed in later INITIAL CONTEXT SETUP REQUEST messages so the
// eNB need not re-fetch it from the UE (TS 23.401).
func handleUECapabilityInfoIndication(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUECapabilityInfoIndication(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcUECapabilityInfoIndication, err)
		return
	}

	ue, ueConn, ok := resolveUE(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcUECapabilityInfoIndication, s1ap.TriggeringInitiatingMessage, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), msg.Diagnostics())

	ue.TouchLastSeen()

	// TS 36.413 §10.3.5: an absent IE leaves the stored capability standing.
	if msg.UERadioCapability != nil {
		ue.RadioCapability = msg.UERadioCapability
	}

	if msg.UERadioCapabilityForPaging != nil {
		ue.RadioCapabilityForPaging = msg.UERadioCapabilityForPaging
	}

	ueConn.Log().Debug("stored UE Radio Capability",
		logger.Bytes(uint64(len(ue.RadioCapability))),
		zap.Int("paging_bytes", len(ue.RadioCapabilityForPaging)))
}
