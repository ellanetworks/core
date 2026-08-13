// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// movingFromEPCInIdleMode reports whether this registration is an inter-system
// change performed in 5GMM-IDLE mode: a mobility registration update reporting
// EMM-REGISTERED, carrying the TRACKING AREA UPDATE REQUEST the UE would have
// sent in S1 mode. A UE that changed system in connected mode sends no such
// container (TS 24.501 §5.5.1.3.2 c), so its arrival is the handover path.
func movingFromEPCInIdleMode(conn *amf.UeConn, req *fgs.RegistrationRequest) bool {
	return conn != nil &&
		conn.RegistrationType5GS == fgs.RegistrationTypeMobilityUpdating &&
		movingFromEPC(req) &&
		len(req.EPSNASMessageContainer) > 0
}

// recoverContextFromEPS fetches the UE's context from the MME and takes it onto
// ue, so the registration proceeds on a context the MME authenticated rather
// than on primary authentication (TS 23.502 §4.11.1.3.3 steps 5a-6,
// TS 33.501 §8.2).
//
// A failure is not fatal: the caller carries on to primary authentication and
// the registration is served as an initial one (TS 24.501 §5.5.1.3.5 b), which
// costs the UE its PDU sessions but not its service.
func recoverContextFromEPS(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest) {
	conn := ue.Conn()
	if conn == nil {
		return
	}

	presented, err := etsi.NewGUTI5GFromNAS(req.MobileIdentity)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Info("inter-system change presents no 5G-GUTI to map back to a 4G one", zap.Error(err))
		return
	}

	resp, err := amfInstance.FetchMMContext(ctx, presented, req.EPSNASMessageContainer)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Info("the MME returned no context for an inter-system change; authenticating the UE instead",
			zap.Error(err))

		return
	}

	// TS 24.501 §5.5.1.3.4 case a: a native 5G context that already verified this
	// request stays current, and the EPS security parameters are discarded. Only
	// the PDN connections are taken from the MME.
	if ue.SecurityContextIsValid() && ue.Supi() == resp.SUPI {
		logger.From(ctx, logger.AmfLog).Info("inter-system change resumed on the UE's native 5G security context",
			logger.SUPI(resp.SUPI.String()))

		conn.ArrivingFromEPS = &resp

		return
	}

	if err := amfInstance.AdoptMMContext(ctx, ue, resp); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to map the EPS context onto 5GS; authenticating the UE instead", zap.Error(err))

		return
	}

	conn.ArrivingFromEPS = &resp

	logger.From(ctx, logger.AmfLog).Info("mapped the UE's EPS security context onto 5GS for an idle-mode change",
		logger.SUPI(resp.SUPI.String()), zap.Int("pdn-connections", len(resp.PDNConnections)))
}

// adoptArrivingSessions takes the PDN connections the MME handed over onto 5GS
// and releases the UE from EPS.
//
// Each session moves with its address and EPS bearer identity but with no user
// plane on the target: an idle move establishes none unless the UE asks for one
// in the Uplink data status IE (TS 23.502 §4.11.1.3.3 step 14). The MME is
// acknowledged once the moves are done, which releases whatever 5GS did not take
// (step 8).
func adoptArrivingSessions(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, conn *amf.UeConn) {
	arriving := conn.ArrivingFromEPS
	if arriving == nil {
		return
	}

	conn.ArrivingFromEPS = nil

	supi := ue.Supi()

	// The MME verified the enclosed TRACKING AREA UPDATE REQUEST against the EPS
	// security context, so this identity is an authenticated one (TS 33.501 §8.2).
	if err := amfInstance.CommitUEIdentity(ctx, ue, amf.MintAuthProofForInterworking()); err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to commit the identity of a UE arriving from EPS", zap.Error(err))
		return
	}

	transferred := make([]uint8, 0, len(arriving.PDNConnections))

	for _, c := range arriving.PDNConnections {
		snssai := c.Snssai

		ref, err := amfInstance.Session.TransferIdleTo5GS(ctx, supi, c.PDUSessionID, c.EPSBearerIdentity, c.APN, &snssai)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("a PDN connection could not move onto 5GS; leaving it behind",
				zap.Error(err), zap.Uint8("pdu_session_id", c.PDUSessionID), zap.String("apn", c.APN))

			continue
		}

		if err := ue.CreateSmContext(c.PDUSessionID, ref, &snssai, c.APN); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to open the SM context of an arriving PDN connection",
				zap.Error(err), zap.Uint8("pdu_session_id", c.PDUSessionID))

			continue
		}

		ue.SetEPSBearerIdentity(c.PDUSessionID, c.EPSBearerIdentity)

		transferred = append(transferred, c.PDUSessionID)
	}

	if err := amfInstance.AckMMContext(ctx, supi, transferred); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("the MME refused the context acknowledgement for an idle-mode change",
			zap.Error(err), logger.SUPI(supi.String()))
	}

	logger.From(ctx, logger.AmfLog).Info("adopted the PDN connections of a UE arriving from EPS in idle mode",
		logger.SUPI(supi.String()), zap.Int("adopted", len(transferred)),
		zap.Int("offered", len(arriving.PDNConnections)))
}
