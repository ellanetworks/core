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

// causeReleaseUnspecified stands in for an omitted Cause IE (TS 36.413 §9.2.1.3,
// CauseRadioNetwork "unspecified").
var causeReleaseUnspecified = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnspecified}

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

	// An omitted Cause is an ignore-criticality absence: the eNB has dropped the radio
	// connection either way, so the release proceeds under a generic cause (§10.3.5).
	cause := causeReleaseUnspecified
	if msg.Cause != nil {
		cause = *msg.Cause
	}

	// A detached connection has no UE but its eNB still holds the context, so answer with
	// a release command (TS 36.413 §8.3.2.2).
	if m.AnswerDetachedRelease(ctx, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID, cause) {
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcUEContextReleaseRequest, s1ap.TriggeringInitiatingMessage, ueAssociated(ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID), msg.Diagnostics())

	ue.TouchLastSeen()

	fields := []zap.Field{
		zap.String("imsi", ue.IMSI()),
		zap.String("cause", mme.S1apCauseName(&cause)),
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

	m.ReleaseUEContext(ctx, ue, cause)
}
