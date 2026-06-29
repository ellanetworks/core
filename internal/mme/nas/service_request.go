// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandleServiceRequest handles a mobile-originated SERVICE REQUEST (TS 24.301)
// carried in an Initial UE Message from an EMM-IDLE UE. It resolves the
// UE by the S-TMSI, verifies the short MAC against the stored NAS context, binds
// the UE to the new S1 association, and re-establishes the S1 context and
// default bearer (ECM-IDLE → ECM-CONNECTED).
func HandleServiceRequest(m *mme.MME, ctx context.Context, conn mme.NasWriter, msg *s1ap.InitialUEMessage) {
	if msg.STMSI == nil {
		logger.MmeLog.Warn("Service Request without an S-TMSI")
		sendServiceReject(m, ctx, conn, msg.ENBUES1APID)

		return
	}

	ue, ok := m.LookupUeByMTMSI(msg.STMSI.MTMSI)
	if !ok || ue.EMMState() != mme.EMMRegistered {
		logger.MmeLog.Info("Service Request for an unknown or deregistered UE",
			zap.Uint32("m-tmsi", msg.STMSI.MTMSI))
		sendServiceReject(m, ctx, conn, msg.ENBUES1APID)

		return
	}

	sr, err := eps.ParseServiceRequest([]byte(msg.NASPDU))
	if err != nil {
		logger.MmeLog.Warn("failed to decode Service Request", zap.Error(err))
		sendServiceReject(m, ctx, conn, msg.ENBUES1APID)

		return
	}

	// An unverified Service Request must not move the UE's S1 connection
	// (TS 24.301 §5.6.1).
	ok, want, expSeq, ul := ue.VerifyServiceRequestShortMAC([]byte(msg.NASPDU)[:2], sr.ShortMAC, sr.SeqShort)
	if !ok {
		logger.MmeLog.Warn("Service Request short-MAC verification failed",
			zap.Uint32("m-tmsi", msg.STMSI.MTMSI),
			zap.String("expected-short-mac", fmt.Sprintf("%x", want)),
			zap.String("received-short-mac", fmt.Sprintf("%x", sr.ShortMAC)),
			zap.Uint8("expected-sequence", expSeq),
			zap.Uint8("received-sequence", sr.SeqShort),
			zap.Uint32("stored-ul-count", ul))

		sendServiceReject(m, ctx, conn, msg.ENBUES1APID)

		return
	}

	if !m.EstablishS1Connection(ue, conn, msg.ENBUES1APID) {
		return
	}

	ue.AdvanceULCount()

	logger.MmeLog.Info("Service Request accepted",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Uint32("enb-ue-id", uint32(ue.S1.ENBUES1APID)),
		zap.String("imsi", ue.IMSI()))

	qos, err := mme.ResolveQoS(m, ctx, ue.IMSI())
	if err != nil {
		logger.MmeLog.Error("failed to resolve subscriber QoS", zap.String("imsi", ue.IMSI()), zap.Error(err))
		return
	}

	sendInitialContextSetup(m, ctx, ue, qos, nil)
}

// sendServiceReject sends a SERVICE REJECT with cause #9 (TS 24.301 §5.6.1.5)
// over a bare connection, so a rejected request never touches a resolved UE.
func sendServiceReject(m *mme.MME, ctx context.Context, conn mme.NasWriter, enbUEID s1ap.ENBUES1APID) {
	c := m.NewConn(conn, enbUEID)
	if c == nil {
		return
	}

	defer m.ReleaseBareConn(c)

	m.SendOverConn(ctx, c, &eps.ServiceReject{Cause: mme.EmmCauseUEIdentityUnderivable})
}
