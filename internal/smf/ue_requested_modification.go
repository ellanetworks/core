// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

type UEIndicatedParams struct {
	Capability           *fgs.GSMCapability
	MaxPacketFilters     *uint16
	IntegrityMaxDataRate *[2]byte
	AlwaysOnGranted      bool
	ModificationDone     bool
}

func requestsQoS(req *fgs.PDUSessionModificationRequest) bool {
	return len(req.RequestedQoSRules) > 0 || len(req.RequestedQoSFlows) > 0
}

func (s *SMF) handleUERequestedModification(ctx context.Context, smContext *SMContext, req *fgs.PDUSessionModificationRequest, pti uint8) (*UpdateResult, error) {
	if req.Cause == nil && requestsQoS(req) {
		logger.WithTrace(ctx, logger.SmfLog).Info("rejecting a UE request to set the session's QoS",
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		n1SmMsg, err := smfNas.BuildGSMPDUSessionModificationReject(fgs.PDUSessionID(smContext.PDUSessionID), naslib.ProcedureTransactionIdentity(pti), fgs.GSMCauseFiveGSQoSNotAccepted)
		if err != nil {
			return nil, fmt.Errorf("build GSM PDUSessionModificationReject failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil
	}

	if req.Cause != nil {
		logger.WithTrace(ctx, logger.SmfLog).Info("the UE reported an error against its own QoS state",
			zap.Stringer("cause", *req.Cause),
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
	}

	alwaysOn := alwaysOnIndication(req.AlwaysOnRequested)

	smContext.recordUEIndicatedParams(req, alwaysOn)

	n1SmMsg, err := smfNas.BuildPDUSessionModificationCommand(smContext.PDUSessionID, pti, nil, nil, nil, 0, nil, alwaysOn)
	if err != nil {
		return nil, fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	smContext.MarkPTIInUse(pti)

	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID

	s.armRetransmit(smContext, s.t3591,
		func() error { return s.amf.ModifyN1N2(context.Background(), supi, pduSessionID, n1SmMsg, nil) },
		func(sc *SMContext) {
			sc.ClearPTIInUse(pti)

			logger.SmfLog.Warn("T3591 expired; UE-requested PDU session modification aborted, session remains active",
				logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
		})

	logger.WithTrace(ctx, logger.SmfLog).Info("accepted a UE-requested PDU session modification",
		zap.Bool("always_on_answered", alwaysOn != nil),
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return &UpdateResult{N1Msg: n1SmMsg}, nil
}

func (smContext *SMContext) recordUEIndicatedParams(req *fgs.PDUSessionModificationRequest, alwaysOn *bool) {
	if req.GSMCapability != nil {
		smContext.ueParams.Capability = req.GSMCapability
	}

	if req.MaxPacketFilters != nil {
		smContext.ueParams.MaxPacketFilters = req.MaxPacketFilters
	}

	if req.IntegrityProtMaxDataRate != nil {
		smContext.ueParams.IntegrityMaxDataRate = req.IntegrityProtMaxDataRate
	}

	smContext.ueParams.AlwaysOnGranted = alwaysOn != nil && *alwaysOn
}

func (smContext *SMContext) UEIndicatedParams() UEIndicatedParams {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return smContext.ueParams
}
