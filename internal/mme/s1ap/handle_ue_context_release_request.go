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

// handleUEContextReleaseRequest handles an eNB-initiated UE Context Release
// Request (inactivity or radio-link failure), starting the S1 release procedure
// (TS 36.413). Whether the context is deleted or retained in ECM-IDLE is decided
// at release-complete from the EMM state.
func handleUEContextReleaseRequest(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseRequest(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcUEContextReleaseRequest, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	fields := []zap.Field{
		zap.String("imsi", ue.IMSI()),
		zap.String("cause", mme.S1apCauseName(&msg.Cause)),
	}

	// A release after the NAS security context is established but before the UE is
	// EMM-REGISTERED aborts an in-progress attach: the eNB dropped the RRC connection
	// before INITIAL CONTEXT SETUP RESPONSE and ATTACH COMPLETE, so the UE restarts the
	// attach. Surface it as a failure.
	if ue.Secured() && ue.EMMState() == mme.EMMRegistrationInitiated {
		icsReceived := false
		if p := m.DefaultPDN(ue); p != nil {
			icsReceived = p.EnbFTEID.TEID != 0
		}

		logger.From(ctx, ue.Conn().Log).Warn("UE Context Release Request aborted an in-progress attach",
			append(fields, zap.Bool("ics-response-received", icsReceived))...)
	} else {
		logger.From(ctx, ue.Conn().Log).Info("UE Context Release Request", fields...)
	}

	// Deactivate before the S1 UE Context Release Command so a concurrent downlink is
	// buffered for paging (TS 23.401 §5.3.5: Release Access Bearers precedes the release
	// command). Only a registered UE transitions to ECM-IDLE.
	if ue.EMMState() == mme.EMMRegistered {
		m.DeactivateAllSessions(ctx, ue)
	}

	m.ReleaseUEContext(ctx, ue, msg.Cause)
}
