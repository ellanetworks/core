// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleUECapabilityInfoIndication stores the UE Radio Capability reported by the
// eNB (TS 36.413), replayed in later INITIAL CONTEXT SETUP REQUEST messages so the
// eNB need not re-fetch it from the UE (TS 23.401).
func handleUECapabilityInfoIndication(m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUECapabilityInfoIndication(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcUECapabilityInfoIndication, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, radio.Conn, s1ap.ProcUECapabilityInfoIndication, ueAssociated(ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID), msg.Diagnostics())

	ue.TouchLastSeen()

	// TS 36.413 §10.3.5: an absent IE leaves the stored capability standing.
	if msg.UERadioCapability != nil {
		ue.RadioCapability = msg.UERadioCapability
	}

	if msg.UERadioCapabilityForPaging != nil {
		ue.RadioCapabilityForPaging = msg.UERadioCapabilityForPaging
	}

	ue.Conn().Log.Info("stored UE Radio Capability",
		zap.Int("bytes", len(ue.RadioCapability)),
		zap.Int("paging-bytes", len(ue.RadioCapabilityForPaging)))
}
