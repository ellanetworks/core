// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandleInitialUEMessage routes a UE's first NAS message on a new S1 association
// (TS 36.413). A SERVICE REQUEST re-establishes an existing EMM-IDLE context
// (resolved by S-TMSI); anything else starts a new one.
func HandleInitialUEMessage(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseInitialUEMessage(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcInitialUEMessage, err)
		return
	}

	nas := []byte(msg.NASPDU)
	if len(nas) > 0 && nas[0]>>4 == uint8(eps.SHTServiceRequest) {
		m.NAS.HandleServiceRequest(ctx, radio.Conn, msg)
		return
	}

	// A bare UE-associated S1-connection tracks the message. The NAS layer binds a
	// persistent context only for an ATTACH REQUEST; a recognised resume is bound
	// below. Anything else leaves the connection bare and releases it so an
	// unauthenticated peer cannot exhaust UE contexts.
	c := m.NewUeConn(radio.Conn, msg.ENBUES1APID)
	if c == nil {
		return
	}

	c.ServingTAI = msg.TAI
	c.UpdateLocation(msg.EUTRANCGI, msg.TAI)

	logger.From(ctx, c.Log).Info("Initial UE Message",
		zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
	)

	m.DropStaleUe(radio.Conn, msg.ENBUES1APID)

	// Optimistic S-TMSI resume: a security-protected message whose S-TMSI resolves a
	// held, secured context is bound to that context only after the message verifies
	// against it (a pure check, not committed here) (TS 24.301 §4.4.4.3). An unverified
	// message cannot move the UE; the NAS layer re-decodes against the bound context to
	// commit the uplink NAS COUNT, so this hint is not authoritative.
	if len(nas) > 0 && nas[0]>>4 != uint8(eps.SHTPlain) && msg.STMSI != nil {
		if ue, ok := m.LookupUeByMTMSI(msg.STMSI.MTMSI); ok && ue.EMMState() == mme.EMMRegistered && ue.Secured() {
			if _, _, err := ue.TryUnprotectUplink(nas); err == nil {
				logger.From(ctx, c.Log).Debug("Initial UE Message: resuming held context",
					zap.Uint32("m-tmsi", msg.STMSI.MTMSI))
				m.AttachUeConn(ue, c)
			}
		}
	}

	m.NAS.HandleNAS(ctx, c, nas)

	if c.UeContext() != nil {
		return
	}

	// No context was bound. A protected TRACKING AREA UPDATE the MME cannot resolve is
	// rejected with EMM cause #9 so the UE re-attaches at once without waiting out T3430
	// (TS 24.301 §5.5.3.2.5); any other message is dropped. The bare connection is
	// released either way.
	if isProtectedTrackingAreaUpdate(nas) {
		metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultReject)
		logger.From(ctx, logger.MmeLog).Info("Tracking Area Update rejected; UE will re-attach",
			zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)))
		c.SendDownlinkMessage(ctx, &eps.TrackingAreaUpdateReject{Cause: mme.EmmCauseUEIdentityUnderivable})
	} else {
		logger.From(ctx, logger.MmeLog).Debug("dropping non-Attach Initial UE Message",
			zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)))
	}

	m.ReleaseBareConn(c)
}

// isProtectedTrackingAreaUpdate reports whether nas is an integrity-protected
// TRACKING AREA UPDATE REQUEST. A ciphered body cannot be peeked, so it never matches.
func isProtectedTrackingAreaUpdate(nas []byte) bool {
	if len(nas) < 6 {
		return false
	}

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil || pd != eps.PDEMM {
		return false
	}

	switch nas[0] >> 4 {
	case uint8(eps.SHTIntegrityProtected), uint8(eps.SHTIntegrityProtectedNewContext):
	default:
		return false
	}

	mt, err := eps.PeekMessageType(nas[6:])

	return err == nil && mt == eps.MsgTrackingAreaUpdateRequest
}
