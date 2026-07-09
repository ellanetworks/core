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

	// TS 23.401 §5.3.5: buffer the downlink (Release Access Bearers, step 2) BEFORE the
	// S1 UE Context Release Command (step 4), so a downlink arriving during the release
	// is buffered for paging rather than forwarded to the eNB bearer being torn down.
	// Only a registered UE goes to ECM-IDLE; an aborted attach or detach is fully
	// released at release-complete. Mirrors the 5G AN-release order (deactivate on the
	// release request, not on completion).
	if ue.EMMState() == mme.EMMRegistered {
		m.DeactivateAllSessions(ctx, ue)
	}

	m.ReleaseUEContext(ctx, ue, msg.Cause)
}
