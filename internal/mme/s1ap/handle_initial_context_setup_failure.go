// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleInitialContextSetupFailure releases the UE locally without a UE Context
// Release Command: the eNB reported it could not set up the context and has already
// released its side (TS 36.413 §8.3.1.3).
func handleInitialContextSetupFailure(m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseInitialContextSetupFailure(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Initial Context Setup Failure", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	logger.MmeLog.Warn("Initial Context Setup Failure",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Any("cause", msg.Cause))

	m.ReleaseUEContextLocally(ue, "initial context setup failure")
}
