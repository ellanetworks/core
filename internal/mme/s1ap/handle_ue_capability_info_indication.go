// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleUECapabilityInfoIndication stores the UE Radio Capability reported by
// the eNB (TS 36.413). The MME keeps the most up-to-date capability and
// replays it in subsequent INITIAL CONTEXT SETUP REQUEST messages so the eNB
// need not re-fetch it from the UE (TS 23.401).
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

	ue.TouchLastSeen()

	ue.RadioCapability = msg.UERadioCapability
	ue.RadioCapabilityForPaging = msg.UERadioCapabilityForPaging
	ue.Conn().Log.Info("stored UE Radio Capability",
		zap.Int("bytes", len(msg.UERadioCapability)),
		zap.Int("paging-bytes", len(msg.UERadioCapabilityForPaging)))
}
