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

// handleENBConfigurationTransfer relays a SON Configuration Transfer IE between
// eNBs (TS 36.413 §8.15/§8.16). Per §8.15.2 the MME "shall transparently transfer
// the SON Configuration Transfer IE towards the eNB indicated in the Target eNB-ID
// IE"; the IE is copied verbatim into an MME CONFIGURATION TRANSFER. Non-UE,
// fire-and-forget: an absent IE or an unconnected target eNB is dropped.
func handleENBConfigurationTransfer(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseENBConfigurationTransfer(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcENBConfigurationTransfer, err)
		return
	}

	if msg.SONConfigurationTransfer == nil {
		return
	}

	target, err := msg.SONConfigurationTransfer.TargetENBID()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not decode Target eNB-ID from SON Configuration Transfer", zap.Error(err))
		return
	}

	targetRadio, ok := m.FindRadioByGlobalENBID(target.GlobalENBID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("SON Configuration Transfer target eNB not connected", zap.String("target-enb", mme.ENBID(target.GlobalENBID)))
		return
	}

	mct := &s1ap.MMEConfigurationTransfer{SONConfigurationTransfer: msg.SONConfigurationTransfer}

	b, err := mct.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal MME Configuration Transfer", zap.Error(err))
		return
	}

	m.SendS1APConn(ctx, targetRadio.Conn, mme.S1APProcedureMMEConfigurationTransfer, b)
}
