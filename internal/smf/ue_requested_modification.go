// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func requestsQoSChange(req *fgs.PDUSessionModificationRequest) bool {
	return len(req.RequestedQoSRules) > 0 || len(req.RequestedQoSFlows) > 0 || len(req.MappedEPSBearerContexts) > 0
}

func qoSChangeRejectCause(req *fgs.PDUSessionModificationRequest) fgs.GSMCause {
	if req.Cause != nil {
		return fgs.GSMCauseRequestRejectedUnspecified
	}

	return fgs.GSMCauseFiveGSQoSNotAccepted
}

func requestsDNSServer(req *fgs.PDUSessionModificationRequest) bool {
	if req.ExtendedPCO == nil {
		return false
	}

	for _, id := range req.ExtendedPCO.ContainerIDs() {
		switch id {
		case naslib.PCOContainerDNSServerIPv4Address, naslib.PCOContainerDNSServerIPv6Address:
			return true
		}
	}

	return false
}

func (smContext *SMContext) dnsForModification(req *fgs.PDUSessionModificationRequest) net.IP {
	if !requestsDNSServer(req) || smContext.PolicyData == nil {
		return nil
	}

	return smContext.PolicyData.DNS
}

func (s *SMF) handleUERequestedModification(ctx context.Context, smContext *SMContext, req *fgs.PDUSessionModificationRequest, pti uint8) (*UpdateResult, error) {
	if smContext.networkProcedureOutstanding() {
		logger.WithTrace(ctx, logger.SmfLog).Info("ignoring a UE-requested PDU session modification that collided with an outstanding network-requested procedure",
			zap.Bool("releasing", smContext.releasing),
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		return nil, nil
	}

	if req.Cause != nil {
		logger.WithTrace(ctx, logger.SmfLog).Info("the UE reported an error against its own QoS state",
			zap.Stringer("cause", *req.Cause),
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
	}

	if requestsQoSChange(req) {
		cause := qoSChangeRejectCause(req)

		logger.WithTrace(ctx, logger.SmfLog).Info("rejecting a UE request to change the session's QoS",
			zap.Stringer("cause", cause),
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		n1SmMsg, err := smfNas.BuildGSMPDUSessionModificationReject(fgs.PDUSessionID(smContext.PDUSessionID), naslib.ProcedureTransactionIdentity(pti), cause)
		if err != nil {
			return nil, fmt.Errorf("build GSM PDUSessionModificationReject failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil
	}

	alwaysOn := alwaysOnIndication(req.AlwaysOnRequested)

	dns := smContext.dnsForModification(req)

	n1SmMsg, err := smfNas.BuildPDUSessionModificationCommand(smContext.PDUSessionID, pti, nil, nil, dns, 0, nil, alwaysOn)
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
		zap.Bool("always_on_answered", alwaysOn != nil), zap.Bool("dns_answered", dns != nil),
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return &UpdateResult{N1Msg: n1SmMsg}, nil
}
