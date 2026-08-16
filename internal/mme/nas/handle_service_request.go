// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandleServiceRequest handles a mobile-originated SERVICE REQUEST, re-establishing
// the S1 context (ECM-IDLE → ECM-CONNECTED) once the short MAC verifies against the
// stored NAS context (TS 24.301).
func HandleServiceRequest(ctx context.Context, m *mme.MME, conn mme.S1APWriter, msg *s1ap.InitialUEMessage) {
	if msg.STMSI == nil {
		logger.From(ctx, logger.MmeLog).Warn("Service Request without an S-TMSI")
		sendServiceReject(ctx, m, conn, msg.ENBUES1APID, eps.EMMCauseUEIdentityCannotBeDerived)

		return
	}

	ue, ok := m.LookupUeByMTMSI(uint32(msg.STMSI.MTMSI))
	if !ok || ue.EMMState() != mme.EMMRegistered {
		logger.From(ctx, logger.MmeLog).Info("Service Request for an unknown or deregistered UE",
			zap.Uint32("m-tmsi", uint32(msg.STMSI.MTMSI)))
		sendServiceReject(ctx, m, conn, msg.ENBUES1APID, eps.EMMCauseUEIdentityCannotBeDerived)

		return
	}

	sr, err := eps.ParseServiceRequest([]byte(msg.NASPDU))
	if !decoded(ctx, "ServiceRequest", err) {
		// Protocol error: a malformed SERVICE REQUEST is answered with EMM cause #96
		// "invalid mandatory information", not #9 (TS 24.301 §5.6.1.7 b).
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Service Request", zap.Error(err))
		sendServiceReject(ctx, m, conn, msg.ENBUES1APID, eps.EMMCauseInvalidMandatoryInformation)

		return
	}

	// An unverified Service Request must not move the UE's S1 connection
	// (TS 24.301 §5.6.1).
	expSeq, ul, err := ue.VerifyServiceRequest(sr)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("Service Request verification failed",
			zap.Uint32("m-tmsi", uint32(msg.STMSI.MTMSI)),
			zap.Uint8("expected-sequence", expSeq),
			zap.Uint8("received-sequence", sr.SeqShort),
			zap.Uint32("stored-ul-count", ul),
			zap.Error(err))

		sendServiceReject(ctx, m, conn, msg.ENBUES1APID, eps.EMMCauseUEIdentityCannotBeDerived)

		return
	}

	c := m.NewUeConn(conn, msg.ENBUES1APID)
	if c == nil {
		return
	}

	// A resume reaches here only after its message was integrity-verified against the
	// held context, so secure exchange is established on the new connection from the
	// outset.
	m.AttachUeConn(ue, c)
	c.MarkSecureExchangeEstablished()
	c.MarkCipheringStarted()

	c.ServingTAI = msg.TAI

	if msg.EUTRANCGI != nil {
		c.UpdateLocation(*msg.EUTRANCGI, msg.TAI)
	}

	ue.AdvanceULCount()
	ue.PinKeNBFreshness()

	logger.From(ctx, logger.MmeLog).Info("Service Request accepted",
		zap.Uint32("enb-ue-id", uint32(c.ENBUES1APID)),
		zap.String("imsi", ue.IMSI()))

	qos, err := mme.ResolveQoS(ctx, m, ue.IMSI())
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to resolve subscriber QoS", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	if ics, carrier, ok := buildInitialContextSetup(ctx, m, ue, c, qos); ok {
		_ = sendInitialContextSetup(ctx, c, ics, carrier, nil)
	}

	// The SERVICE REQUEST carries no accept message to convey a fresh GUTI, so reassign
	// one with the standalone reallocation procedure — identity confidentiality on return
	// from idle (TS 24.301 §5.4.1).
	m.SendGUTIReallocationCommand(ctx, ue)
}

// sendServiceReject sends a SERVICE REJECT with the given EMM cause over a bare connection,
// so a rejected request never touches a resolved UE (TS 24.301 §5.6.1.5, §5.6.1.7).
func sendServiceReject(ctx context.Context, m *mme.MME, conn mme.S1APWriter, enbUEID s1ap.ENBUES1APID, cause eps.EMMCause) {
	c := m.NewUeConn(conn, enbUEID)
	if c == nil {
		return
	}

	defer m.ReleaseBareConn(c)

	c.SendDownlinkMessage(ctx, &eps.ServiceReject{Cause: cause})
}
