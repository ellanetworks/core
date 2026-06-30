// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func handleESM(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	mt, err := eps.PeekESMMessageType(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to read ESM message type", zap.Error(err))
		return
	}

	ctx, span := mme.Tracer.Start(ctx, "mme/esm",
		trace.WithAttributes(attribute.Int("esm.message_type", int(mt))))
	defer span.End()

	switch mt {
	case eps.MsgPDNConnectivityRequest:
		handlePDNConnectivityRequest(m, ctx, ue, plain)
	case eps.MsgPDNDisconnectRequest:
		handlePDNDisconnectRequest(m, ctx, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextAccept:
		handleActivateDefaultBearerAccept(m, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextReject:
		handleActivateDefaultBearerReject(m, ue, plain)
	case eps.MsgDeactivateEPSBearerContextAccept:
		handleDeactivateBearerAccept(m, ctx, ue, plain)
	case eps.MsgModifyEPSBearerContextAccept:
		handleModifyBearerAccept(m, ue, plain)
	case eps.MsgModifyEPSBearerContextReject:
		handleModifyBearerReject(m, ue, plain)
	default:
		logger.MmeLog.Warn("unhandled ESM message", zap.Int("message-type-value", int(mt)))
	}
}

// handleModifyBearerAccept commits the new bearer configuration once the UE accepts
// the in-place modification (TS 24.301 §6.4.2.3). The accept's EPS bearer identity
// selects the PDN connection, so an additional PDN commits to the right bearer.
func handleModifyBearerAccept(m *mme.MME, ue *mme.UeContext, plain []byte) {
	p := m.DefaultPDN(ue)
	if accept, err := eps.ParseModifyEPSBearerContextAccept(plain); err == nil {
		if named := m.LookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return
	}

	m.StopESMGuard(p)

	if !ue.CommitBearerModification(p) {
		return
	}

	logger.MmeLog.Info("EPS bearer modified in place",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
}

// handleModifyBearerReject abandons the modification when the UE rejects it
// (TS 24.301 §6.4.2.4), leaving the stored config stale so the backstop retries.
func handleModifyBearerReject(m *mme.MME, ue *mme.UeContext, plain []byte) {
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

	logger.MmeLog.Warn("UE rejected EPS bearer modification",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()))
}

// handleDeactivateBearerAccept finalises an EPS bearer deactivation. A deactivation
// triggered by a UE PDN disconnect releases only that PDN connection and leaves
// the UE connected (TS 24.301 §6.5.2). A deactivation with reactivation requested
// for the default bearer instead releases the S1 context so the UE re-attaches
// and picks up the new data-network configuration (TS 24.301 §6.4.4.2).
func handleDeactivateBearerAccept(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	p := m.DefaultPDN(ue)
	if accept, err := eps.ParseDeactivateEPSBearerContextAccept(plain); err == nil {
		if named := m.LookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return
	}

	m.StopESMGuard(p)

	// Reactivating the attach PDN's default bearer detaches the UE so it re-attaches
	// with the new configuration (TS 24.301 §6.4.4.2); any other case releases just
	// that PDN connection and leaves the UE connected.
	releaseOnly := ue.BearerReleaseOnly(p)

	if releaseOnly {
		logger.MmeLog.Info("PDN connection released",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
		m.ReleasePDN(ue, p)

		return
	}

	ue.ClearDeactivating(p)

	ue.SetEMMState(mme.EMMDeregistered)
	m.ReleaseAllSessions(ue)

	logger.MmeLog.Info("EPS bearer deactivated for reactivation; UE will re-attach",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()))
	m.ReleaseUEContext(ctx, ue, mme.CauseNASNormalRelease)
}
