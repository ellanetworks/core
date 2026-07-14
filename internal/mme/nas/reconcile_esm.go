// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func handleESM(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	mt, err := eps.PeekESMMessageType(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read ESM message type", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	ctx, span := mme.Tracer.Start(ctx, "mme/esm",
		trace.WithAttributes(attribute.Int("esm.message_type", int(mt))))
	defer span.End()

	switch mt {
	case eps.MsgPDNConnectivityRequest:
		return handlePDNConnectivityRequest(ctx, m, ue, plain)
	case eps.MsgPDNDisconnectRequest:
		return handlePDNDisconnectRequest(ctx, m, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextAccept:
		return handleActivateDefaultBearerAccept(m, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextReject:
		return handleActivateDefaultBearerReject(ctx, m, ue, plain)
	case eps.MsgDeactivateEPSBearerContextAccept:
		return handleDeactivateBearerAccept(ctx, m, ue, plain)
	case eps.MsgModifyEPSBearerContextAccept:
		return handleModifyBearerAccept(m, ue, plain)
	case eps.MsgModifyEPSBearerContextReject:
		return handleModifyBearerReject(m, ue, plain)
	case eps.MsgESMStatus:
		return handleESMStatus(ctx, m, ue, plain)
	default:
		// TS 24.301 §7.4: an ESM message type not implemented is answered with an ESM STATUS
		// #97 "message type non-existent or not implemented" — the MME hosts ESM, so unlike
		// the AMF (which relays 5GSM to the SMF) it emits the STATUS itself.
		logger.From(ctx, logger.MmeLog).Warn("unhandled ESM message", zap.Int("message-type-value", int(mt)))
		return nasreply.StatusSM(nasreply.CauseMessageTypeNotImplemented)
	}
}

// handleModifyBearerAccept commits the new bearer configuration once the UE accepts
// the in-place modification (TS 24.301 §6.4.2.3). The accept's EPS bearer identity
// selects the PDN connection, so an additional PDN commits to the right bearer.
func handleModifyBearerAccept(m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	p := m.DefaultPDN(ue)
	if accept, err := eps.ParseModifyEPSBearerContextAccept(plain); err == nil {
		if named := m.LookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	if !ue.CommitBearerModification(p) {
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().Log.Info("EPS bearer modified in place", zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))

	return nasreply.Handled()
}

// handleModifyBearerReject abandons the modification when the UE rejects it
// (TS 24.301 §6.4.2.4), leaving the stored config stale so the backstop retries.
func handleModifyBearerReject(m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	p := m.DefaultPDN(ue)
	if rej, err := eps.ParseModifyEPSBearerContextReject(plain); err == nil {
		if named := m.LookupPDN(ue, rej.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p != nil {
		m.StopESMGuard(p)
		ue.ClearPendingModify(p)
	}

	ue.Conn().Log.Warn("UE rejected EPS bearer modification", zap.String("imsi", ue.IMSI()))

	return nasreply.Handled()
}

// handleDeactivateBearerAccept finalises an EPS bearer deactivation. A deactivation
// triggered by a UE PDN disconnect releases only that PDN connection and leaves
// the UE connected (TS 24.301 §6.5.2). A deactivation with reactivation requested
// for the default bearer releases the S1 context so the UE re-attaches
// and picks up the new data-network configuration (TS 24.301 §6.4.4.2).
func handleDeactivateBearerAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	p := m.DefaultPDN(ue)
	if accept, err := eps.ParseDeactivateEPSBearerContextAccept(plain); err == nil {
		if named := m.LookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	releaseOnly := ue.BearerReleaseOnly(p)
	ue.ClearDeactivating(p)

	if releaseOnly {
		logger.From(ctx, logger.MmeLog).Info("PDN connection released", zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
	} else {
		logger.From(ctx, logger.MmeLog).Info("EPS bearer deactivated for reactivation; UE will re-attach", zap.String("imsi", ue.IMSI()))
	}

	m.DeactivatePDNLocally(ctx, ue, p)

	return nasreply.Handled()
}

// handleESMStatus processes a received ESM STATUS (TS 24.301 §6.7): it aborts the
// ongoing ESM procedure for the indicated bearer and stops its guard; ESM cause
// #43 (invalid EPS bearer identity) also deactivates the bearer locally. A valid
// ESM STATUS is never answered with another STATUS.
func handleESMStatus(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	status, err := eps.ParseESMStatus(plain)
	if err != nil {
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	logger.From(ctx, logger.MmeLog).Warn("received ESM STATUS",
		zap.String("imsi", ue.IMSI()),
		zap.Uint8("ebi", status.EPSBearerIdentity),
		zap.Uint8("esm-cause", status.ESMCause))

	p := m.LookupPDN(ue, status.EPSBearerIdentity)
	if p == nil {
		return nasreply.Handled()
	}

	m.StopESMGuard(p)
	ue.ClearPendingModify(p)
	ue.ClearDeactivating(p)

	if status.ESMCause == esmCauseInvalidEPSBearerIdentity {
		m.DeactivatePDNLocally(ctx, ue, p)
	}

	return nasreply.Handled()
}
