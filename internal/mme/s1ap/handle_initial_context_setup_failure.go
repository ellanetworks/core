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

// handleInitialContextSetupFailure releases the UE locally without a UE Context
// Release Command: the eNB reported it could not set up the context and has already
// released its side (TS 36.413 §8.3.1.3).
func handleInitialContextSetupFailure(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseInitialContextSetupFailure(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcInitialContextSetup, err)
		return
	}

	ue, ueConn, ok := resolveUEIDs(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcInitialContextSetup, s1ap.TriggeringUnsuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), msg.Diagnostics())

	ue.TouchLastSeen()

	fields := []zap.Field{zap.Uint32("mme-ue-id", uint32(*msg.MMEUES1APID))}
	if msg.Cause != nil {
		fields = append(fields, zap.String("cause", mme.S1apCauseName(msg.Cause)))
	}

	logger.MmeLog.Warn("Initial Context Setup Failure", fields...)

	m.ReleaseUEContextLocally(ue, "initial context setup failure")
}
