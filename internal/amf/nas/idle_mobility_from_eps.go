// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func movingFromEPCInIdleMode(conn *amf.UeConn, req *fgs.RegistrationRequest) bool {
	return conn != nil &&
		conn.RegistrationType5GS == fgs.RegistrationTypeMobilityUpdating &&
		movingFromEPC(req) &&
		len(req.EPSNASMessageContainer) > 0
}

func citedNgKsiIdentifiesCurrentContext(ue *amf.UeContext, req *fgs.RegistrationRequest) bool {
	held := ue.NgKsi()

	if req.NgKSI.Mapped != (held.Tsc == models.ScTypeMapped) {
		return false
	}

	return int32(req.NgKSI.Value) == held.Ksi
}

func recoverContextFromEPS(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest, integrityVerified bool) {
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

	if integrityVerified && ue.SecurityContextIsValid() && ue.Supi() == resp.SUPI && citedNgKsiIdentifiesCurrentContext(ue, req) {
		logger.From(ctx, logger.AmfLog).Info("inter-system change resumed on the UE's native 5G security context",
			logger.SUPI(resp.SUPI.String()))

		conn.EPSArrival = &amf.EPSArrival{Sessions: &interworking.ArrivingSessions{PDN: resp.PDNConnections}}

		return
	}

	if err := amfInstance.AdoptMMContext(ctx, ue, resp); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to map the EPS context onto 5GS; authenticating the UE instead", zap.Error(err))

		return
	}

	conn.EPSArrival = &amf.EPSArrival{
		Sessions:                 &interworking.ArrivingSessions{PDN: resp.PDNConnections},
		NeedsSecurityModeControl: true,
	}

	logger.From(ctx, logger.AmfLog).Info("mapped the UE's EPS security context onto 5GS for an idle-mode change",
		logger.SUPI(resp.SUPI.String()), zap.Int("pdn-connections", len(resp.PDNConnections)))
}

func adoptArrivingSessions(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, conn *amf.UeConn) bool {
	arriving := conn.EPSArrival.ArrivingSessions()
	if arriving == nil {
		return true
	}

	conn.EPSArrival.Sessions = nil

	supi := ue.Supi()

	if err := amfInstance.CommitUEIdentity(ctx, ue, amf.MintAuthProofForInterworking()); err != nil {
		abortRegistration(ctx, amfInstance, ue, "commit the identity of a UE arriving from EPS", err)

		return false
	}

	transferred := make([]uint8, 0, len(arriving.PDN))

	for _, c := range arriving.PDN {
		snssai := c.Snssai

		ref, err := amfInstance.Session.TransferIdleTo5GS(ctx, supi, c.PDUSessionID, c.EPSBearerIdentity, c.APN, &snssai)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("a PDN connection could not move onto 5GS; leaving it behind",
				zap.Error(err), zap.Uint8("pdu_session_id", c.PDUSessionID), zap.String("apn", c.APN))

			continue
		}

		if err := ue.CreateSmContext(c.PDUSessionID, ref, &snssai, c.APN); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to open the SM context of an arriving PDN connection; releasing the session it moved",
				zap.Error(err), zap.Uint8("pdu_session_id", c.PDUSessionID))

			if err := amfInstance.Session.ReleaseSmContext(ctx, ref); err != nil {
				logger.From(ctx, logger.AmfLog).Warn("failed to release the session of a PDN connection the AMF could not adopt",
					zap.Error(err), zap.Uint8("pdu_session_id", c.PDUSessionID))
			}

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
		zap.Int("offered", len(arriving.PDN)))

	return true
}
