// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
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
	req *fgs.PDUSessionEstablishmentRequest,
	pti uint8,
) (string, []byte, error) {
	policy, err := s.GetSessionPolicy(ctx, supi, snssai, dnn)
	if err != nil {
		return "", rejectTransfer5GS(pduSessionID, pti, establishmentRejectCause(err)),
			fmt.Errorf("failed to find subscriber policy for a session move: %w", err)
	}

	move := transferRequest{Access: Access5G, Dnn: dnn, Snssai: snssai, Policy: policy}

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
		sc.abandonTransfer()

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

	if err := s.sendPduSessionEstablishmentAccept(ctx, sc, policy, pco, addrs, pti, nil, alwaysOnIndication(req.AlwaysOnRequested)); err != nil {
		s.abandonAndRelease(ctx, sc)

		return "", nil, fmt.Errorf("failed to send the establishment accept for a moved session: %w", err)
	}

	return sc.Ref, nil, nil
}

func (s *SMF) abandonAndRelease(ctx context.Context, sc *SMContext) {
	sc.abandonTransfer()

	sc.Mutex.Lock()
	s.RemoveSession(ctx, sc.Ref)
	sc.Mutex.Unlock()
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

// #54 claims the network has no information about the session, which is untrue
// for one the anchor holds but cannot move, so those draw the retryable #26.
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
