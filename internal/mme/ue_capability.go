// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleUECapabilityInfoIndication stores the UE Radio Capability reported by
// the eNB (TS 36.413). The MME keeps the most up-to-date capability and
// replays it in subsequent INITIAL CONTEXT SETUP REQUEST messages so the eNB
// need not re-fetch it from the UE (TS 23.401).
func (m *MME) handleUECapabilityInfoIndication(conn nasWriter, value []byte) {
	msg, err := s1ap.ParseUECapabilityInfoIndication(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode UE Capability Info Indication", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.radioCapability = msg.UERadioCapability
	logger.MmeLog.Info("stored UE Radio Capability",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Int("bytes", len(msg.UERadioCapability)))
}
