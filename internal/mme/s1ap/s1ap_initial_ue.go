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
// (TS 36.413). A SERVICE REQUEST re-establishes an existing EMM-IDLE
// context (resolved by S-TMSI); anything else (an Attach Request) starts a new
// one.
func HandleInitialUEMessage(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	msg, err := s1ap.ParseInitialUEMessage(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Initial UE Message", zap.Error(err))
		return
	}

	nas := []byte(msg.NASPDU)
	if len(nas) > 0 && nas[0]>>4 == uint8(eps.SHTServiceRequest) {
		m.NAS.HandleServiceRequest(ctx, conn, msg)
		return
	}

	// A security-protected NAS message from a UE that presents its S-TMSI is a
	// resume in an existing security context (e.g. a TAU from idle). The message
	// is authenticated against the resolved context before the context is bound to
	// the requesting association (TS 24.301 §4.4.4.3), so an unverified message
	// cannot move the UE. A UE without a resolvable context (e.g. after an MME
	// restart) falls through to a fresh context below.
	if len(nas) > 0 && nas[0]>>4 != uint8(eps.SHTPlain) && msg.STMSI != nil {
		if ue, ok := m.LookupUeByMTMSI(msg.STMSI.MTMSI); ok && ue.EMMState() == mme.EMMRegistered && ue.Secured() {
			plain, count, err := ue.TryUnprotectUplink(nas)
			if err != nil {
				logger.MmeLog.Warn("Initial UE Message (resume) failed integrity check",
					zap.Uint32("m-tmsi", msg.STMSI.MTMSI))

				return
			}

			ue.TouchLastSeen()
			m.EstablishS1Connection(ue, conn, msg.ENBUES1APID)
			ue.CommitUplinkCount(count)

			logger.MmeLog.Info("Initial UE Message (resume)",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
				zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
			)

			m.NAS.DispatchEMM(ctx, ue, plain, true)

			return
		}
	}

	// A fresh connection is tracked by a bare UE-associated S1-connection. A
	// persistent UE context is bound to it only when its first NAS message is an
	// ATTACH REQUEST; any other (or malformed) initial message releases the bare
	// connection without binding one, so an unauthenticated peer cannot exhaust UE
	// contexts.
	c := m.NewConn(conn, msg.ENBUES1APID)

	if !isInitialAttach(nas) {
		// A protected TRACKING AREA UPDATE the MME cannot resolve (e.g. a periodic
		// update after an MME restart) is rejected with EMM cause #9 over the bare
		// connection, so the UE re-attaches at once instead of waiting out T3430
		// (TS 24.301 §5.5.3.2.5).
		if isProtectedTrackingAreaUpdate(nas) {
			metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultReject)
			logger.MmeLog.Info("Tracking Area Update rejected; UE will re-attach",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)))
			m.SendOverConn(ctx, c, &eps.TrackingAreaUpdateReject{Cause: mme.EmmCauseUEIdentityUnderivable})
		} else {
			logger.MmeLog.Debug("dropping non-Attach Initial UE Message",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)))
		}

		m.ReleaseBareConn(c)

		return
	}

	m.DropStaleUe(conn, msg.ENBUES1APID)
	ue := m.BindConn(c)

	logger.MmeLog.Info("Initial UE Message",
		zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
	)

	m.NAS.HandleNAS(ctx, ue, nas)
}

// isInitialAttach reports whether a fresh connection's first NAS message is an
// ATTACH REQUEST — the only message warranting a new UE context (TS 24.301):
// plain for an IMSI or foreign-GUTI attach, or integrity-only for a native-GUTI
// re-attach whose body is readable without a security context. A ciphered or
// non-EMM message cannot be an initial attach the network can act on.
func isInitialAttach(nas []byte) bool {
	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil || pd != eps.PDEMM {
		return false
	}

	body := nas

	switch nas[0] >> 4 {
	case uint8(eps.SHTPlain):
	case uint8(eps.SHTIntegrityProtected), uint8(eps.SHTIntegrityProtectedNewContext):
		if len(nas) < 6 {
			return false
		}

		body = nas[6:]
	default:
		return false
	}

	mt, err := eps.PeekMessageType(body)

	return err == nil && mt == eps.MsgAttachRequest
}

// isProtectedTrackingAreaUpdate reports whether nas is an integrity-protected
// (peekable) TRACKING AREA UPDATE REQUEST. An idle UE sends this from a security
// context the MME may have lost (e.g. after a restart); when the context is
// unresolvable the MME rejects it so the UE re-attaches (TS 24.301 §5.5.3.2.5). A
// ciphered body cannot be peeked, so it is not matched.
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

// handleUplinkNASTransport routes an uplink NAS message to its UE context
// (TS 36.413).
func handleUplinkNASTransport(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	msg, err := s1ap.ParseUplinkNASTransport(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Uplink NAS Transport", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	m.NAS.HandleNAS(ctx, ue, []byte(msg.NASPDU))
}
