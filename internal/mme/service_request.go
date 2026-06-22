// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// onServiceRequest handles a mobile-originated SERVICE REQUEST (TS 24.301)
// carried in an Initial UE Message from an EMM-IDLE UE. It resolves the
// UE by the S-TMSI, verifies the short MAC against the stored NAS context, binds
// the UE to the new S1 association, and re-establishes the S1 context and
// default bearer (ECM-IDLE → ECM-CONNECTED).
func (m *MME) onServiceRequest(ctx context.Context, conn nasWriter, msg *s1ap.InitialUEMessage) {
	if msg.STMSI == nil {
		logger.MmeLog.Warn("Service Request without an S-TMSI")
		m.sendServiceReject(ctx, conn, msg.ENBUES1APID, emmCauseUEIdentityUnderivable)

		return
	}

	ue, ok := m.lookupUeByMTMSI(msg.STMSI.MTMSI)
	if !ok || ue.emmState.load() != EMMRegistered {
		logger.MmeLog.Info("Service Request for an unknown or deregistered UE",
			zap.Uint32("m-tmsi", msg.STMSI.MTMSI))
		m.sendServiceReject(ctx, conn, msg.ENBUES1APID, emmCauseUEIdentityUnderivable)

		return
	}

	// The UE returns from ECM-IDLE on a new UE-associated logical S1-connection;
	// allocate a fresh MME-UE-S1AP-ID for it (TS 36.413). Every downlink
	// below — reject or Initial Context Setup — is then deliverable on it.
	m.establishS1Connection(ue, conn, msg.ENBUES1APID)

	sr, err := eps.ParseServiceRequest([]byte(msg.NASPDU))
	if err != nil {
		logger.MmeLog.Warn("failed to decode Service Request", zap.Error(err))
		m.sendDownlinkMessage(ctx, ue, &eps.ServiceReject{Cause: emmCauseUEIdentityUnderivable})

		return
	}

	// Verify the short MAC over the 2-octet header at the expected uplink NAS
	// COUNT; the truncated sequence must also match (TS 24.301).
	want, err := eps.ServiceRequestShortMAC([]byte(msg.NASPDU)[:2], ue.knasInt, ue.ulCount,
		nascommon.DirectionUplink, integrityAlg(ue.eia))
	if err != nil || want != sr.ShortMAC || uint8(ue.ulCount)&0x1f != sr.SeqShort {
		logger.MmeLog.Warn("Service Request short-MAC verification failed",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
			zap.String("expected-short-mac", fmt.Sprintf("%x", want)),
			zap.String("received-short-mac", fmt.Sprintf("%x", sr.ShortMAC)),
			zap.Uint8("expected-sequence", uint8(ue.ulCount)&0x1f),
			zap.Uint8("received-sequence", sr.SeqShort),
			zap.Uint32("stored-ul-count", ue.ulCount))

		m.sendDownlinkMessage(ctx, ue, &eps.ServiceReject{Cause: emmCauseUEIdentityUnderivable})

		return
	}

	ue.ulCount++
	ue.ecmState.store(ECMConnected)

	logger.MmeLog.Info("Service Request accepted",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Uint32("enb-ue-id", uint32(ue.ENBUES1APID)),
		zap.String("imsi", ue.imsi))

	qos, err := m.resolveQoS(ctx, ue.imsi)
	if err != nil {
		logger.MmeLog.Error("failed to resolve subscriber QoS", zap.String("imsi", ue.imsi), zap.Error(err))
		return
	}

	m.sendInitialContextSetup(ctx, ue, qos, nil)
}

// sendServiceReject sends a SERVICE REJECT (TS 24.301) on the given
// association via a transient context, prompting the UE to re-attach.
func (m *MME) sendServiceReject(ctx context.Context, conn nasWriter, enbUEID s1ap.ENBUES1APID, cause uint8) {
	ue := m.newUe(conn, enbUEID)
	defer m.removeUe(ue.MMEUES1APID)

	m.sendDownlinkMessage(ctx, ue, &eps.ServiceReject{Cause: cause})
}
