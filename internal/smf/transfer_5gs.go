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

// isTransferRequest reports whether the UE is asking to move a session it holds
// on the other access rather than establish a new one. Both "existing" types
// name such a session (TS 24.501 §6.4.1.7 d); this network holds no emergency
// session, so the emergency form always resolves to nothing and draws #54.
func isTransferRequest(t fgs.RequestType) bool {
	return t == fgs.RequestTypeExistingPDUSession || t == fgs.RequestTypeExistingEmergencyPDUSession
}

// transferTo5GS moves an existing PDN connection onto 5GS in answer to a PDU
// SESSION ESTABLISHMENT REQUEST with request type "existing PDU session"
// (TS 23.502 §4.11.2.3). The UE keeps its address because the anchor keeps the
// session: nothing is established, the existing one changes access.
//
// The Establishment Accept is built out of what a move preserves — the UE
// address, the negotiated PDU session type and the slice — so the UE sees the
// session it already had. The downlink stays on EPS until the gNB's N3 endpoint
// arrives in the N2 resource setup response, which is where the move commits.
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

	// The accept carries the target access's policy: the UE is told the QoS rule
	// and Session-AMBR it will run under on 5GS, which the commit then programs.
	// No #50/#51 narrowing cause — the PDU session type is the one the session was
	// established with, not one negotiated now.
	if err := s.sendPduSessionEstablishmentAccept(ctx, sc, policy, pco, addrs, pti, nil, alwaysOnIndication(req.AlwaysOnRequested)); err != nil {
		// By now neither access holds a usable reference: the MME has not been told
		// to forget the connection, but the AMF never received a ref for it, so
		// nothing could release the session, its tunnel or its lease. Release it and
		// let the UE re-establish.
		s.abandonAndRelease(ctx, sc)

		return "", nil, fmt.Errorf("failed to send the establishment accept for a moved session: %w", err)
	}

	return sc.Ref, nil, nil
}

// abandonAndRelease drops a prepared move and releases the session with it.
// Caller must not hold sc.Mutex.
func (s *SMF) abandonAndRelease(ctx context.Context, sc *SMContext) {
	sc.abandonTransfer()

	sc.Mutex.Lock()
	s.RemoveSession(ctx, sc.Ref)
	sc.Mutex.Unlock()
}

// rejectTransfer5GS renders a refused move as a PDU SESSION ESTABLISHMENT
// REJECT. A reject that cannot be built leaves the UE to time out, which is
// worse than an unbuilt message but not something the SMF can do anything about.
func rejectTransfer5GS(pduSessionID, pti uint8, cause fgs.GSMCause) []byte {
	rsp, err := smfNas.BuildGSMPDUSessionEstablishmentReject(fgs.PDUSessionID(pduSessionID), nas.ProcedureTransactionIdentity(pti), cause)
	if err != nil {
		logger.SmfLog.Error("failed to build the establishment reject for a refused session move",
			zap.Error(err), logger.PDUSessionID(pduSessionID))

		return nil
	}

	return rsp
}

// transferRejectCause maps a refused move onto the 5GSM cause the UE is told.
//
// #54 "PDU session does not exist" says the network has no information about the
// session, on which the UE establishes a new one instead of retrying
// (TS 24.501 §6.4.1.7 d). It would be untrue for a session the anchor does hold
// but cannot move as asked — a mismatched data network or slice, a move already
// in flight, a session being released — so those draw #26 "insufficient
// resources", which is retryable and is what the EPS side reports for the same
// case.
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
