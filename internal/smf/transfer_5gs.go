// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	smfNgap "github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func isTransferRequest(t fgs.RequestType) bool {
	return t == fgs.RequestTypeExistingPDUSession || t == fgs.RequestTypeExistingEmergencyPDUSession
}

func (s *SMF) transferTo5GS(
	ctx context.Context,
	supi etsi.SUPI,
	pduSessionID uint8,
	dnn string,
	snssai *models.Snssai,
	requestType fgs.RequestType,
	req *fgs.PDUSessionEstablishmentRequest,
	pti uint8,
	epsBearerIdentity uint8,
) (string, []byte, error) {
	if requestType == fgs.RequestTypeExistingEmergencyPDUSession {
		return "", rejectTransfer5GS(pduSessionID, pti, fgs.GSMCausePDUSessionDoesNotExist),
			fmt.Errorf("no emergency PDU session to move onto 5GS")
	}

	policy, err := s.GetSessionPolicy(ctx, supi, snssai, dnn)
	if err != nil {
		return "", rejectTransfer5GS(pduSessionID, pti, establishmentRejectCause(err)),
			fmt.Errorf("failed to find subscriber policy for a session move: %w", err)
	}

	move := transferRequest{Access: Access5G, EBI: epsBearerIdentity, Dnn: dnn, Snssai: snssai, Policy: policy}

	sc, err := s.findTransferable(supi, pduSessionID, move)
	if err != nil {
		return "", rejectTransfer5GS(pduSessionID, pti, transferRejectCause(err)),
			fmt.Errorf("no session to move onto 5GS: %w", err)
	}

	if err := s.prepareTransfer(sc, move); err != nil {
		return "", rejectTransfer5GS(pduSessionID, pti, transferRejectCause(err)),
			fmt.Errorf("failed to prepare a session move onto 5GS: %w", err)
	}

	pco, err := parsePDUSessionRequest(req)
	if err != nil {
		sc.abandonTransferTo(Access5G)

		return "", rejectTransfer5GS(pduSessionID, pti, fgs.GSMCauseRequestRejectedUnspecified),
			fmt.Errorf("parse PDU session request failed: %w", err)
	}

	sc.Mutex.Lock()
	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: fgs.PDUSessionType(sc.PDUSessionType),
		IPv4Address:    sc.PDUIPV4Address,
		IPv6IID:        sc.IPv6IID,
	}
	sc.Mutex.Unlock()

	logger.WithTrace(ctx, logger.SmfLog).Info("moving a PDN connection onto 5GS",
		logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID), zap.String("dnn", dnn))

	if err := s.sendPduSessionEstablishmentAccept(ctx, sc, policy, pco, addrs, pti, nil, alwaysOnIndication(req.AlwaysOnRequested), epsBearerIdentity); err != nil {
		sc.abandonTransferTo(Access5G)

		return "", nil, fmt.Errorf("failed to send the establishment accept for a moved session: %w", err)
	}

	return sc.Ref, nil, nil
}

func (s *SMF) PrepareSmContextFromEPS(ctx context.Context, supi etsi.SUPI, pduSessionID, epsBearerIdentity uint8, dnn string, snssai *models.Snssai) (ref string, n2 []byte, err error) {
	defer func() { recordSessionEstablishment(metrics.RAT5G, err) }()

	ctx, span := tracer.Start(ctx, "smf/prepare_sm_context_from_eps",
		trace.WithAttributes(
			attribute.String("ue.supi", supi.String()),
			attribute.Int("smf.pdu_session_id", int(pduSessionID)),
			attribute.Int("eps.bearer_id", int(epsBearerIdentity)),
			attribute.String("smf.dnn", dnn),
		),
	)
	defer span.End()

	policy, err := s.GetSessionPolicy(ctx, supi, snssai, dnn)
	if err != nil {
		return "", nil, fmt.Errorf("no policy for a PDN connection arriving on 5GS: %w", err)
	}

	move := transferRequest{Access: Access5G, EBI: epsBearerIdentity, Dnn: dnn, Snssai: snssai, Policy: policy}

	sc, err := s.findTransferable(supi, pduSessionID, move)
	if err != nil {
		return "", nil, fmt.Errorf("no PDN connection to move onto 5GS: %w", err)
	}

	if err := s.prepareTransfer(sc, move); err != nil {
		return "", nil, fmt.Errorf("failed to prepare a PDN connection move onto 5GS: %w", err)
	}

	n2, err = handoverRequestTransferForArrival(sc, epsBearerIdentity, policy)
	if err != nil {
		sc.abandonTransferTo(Access5G)

		return "", nil, err
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("admitting a PDN connection handed over from EPS",
		logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID),
		zap.Uint8("ebi", epsBearerIdentity), zap.String("dnn", dnn))

	return sc.Ref, n2, nil
}

func handoverRequestTransferForArrival(sc *SMContext, epsBearerIdentity uint8, target *Policy) ([]byte, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Tunnel == nil {
		return nil, fmt.Errorf("%w: PDU session %d has no tunnel to report", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if sc.EBI != epsBearerIdentity {
		return nil, fmt.Errorf("%w: PDU session %d is EPS bearer %d, not %d", ErrSessionNotMovable, sc.PDUSessionID, sc.EBI, epsBearerIdentity)
	}

	policy := transferPolicy(sc.PolicyData, target)

	n2, err := smfNgap.BuildHandoverRequestTransfer(&policy.Ambr, &policy.QosData,
		sc.Tunnel.N3TEID, sc.Tunnel.N3IPv4, sc.Tunnel.N3IPv6,
		nasToNgapPDUSessionType(sc.PDUSessionType), &epsBearerIdentity)
	if err != nil {
		return nil, fmt.Errorf("build the Handover Request transfer for an arriving PDN connection: %w", err)
	}

	return n2, nil
}

func rejectTransfer5GS(pduSessionID, pti uint8, cause fgs.GSMCause) []byte {
	rsp, err := smfNas.BuildGSMPDUSessionEstablishmentReject(fgs.PDUSessionID(pduSessionID), nas.ProcedureTransactionIdentity(pti), cause)
	if err != nil {
		logger.SmfLog.Error("failed to build the establishment reject for a refused session move",
			zap.Error(err), logger.PDUSessionID(pduSessionID))

		return nil
	}

	return rsp
}

func transferRejectCause(err error) fgs.GSMCause {
	switch {
	case errors.Is(err, models.ErrSessionNotTransferable):
		return fgs.GSMCausePDUSessionDoesNotExist
	case errors.Is(err, models.ErrSessionOnOtherDNN), errors.Is(err, models.ErrSessionNotMovable):
		return fgs.GSMCauseInsufficientResources
	default:
		return fgs.GSMCauseRequestRejectedUnspecified
	}
}
