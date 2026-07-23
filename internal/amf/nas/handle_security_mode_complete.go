// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// TS 33.501
func handleSecurityModeComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	if step := ue.RegStep(); step != amf.RegStepSecurityMode {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Security Mode Complete message outside the security mode exchange", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return nasreply.Handled()
	}

	conn.StopNASGuard()

	conn.Parent().EndKeyChainProc(procedure.SecurityMode)

	msg, err := fgs.ParseSecurityModeComplete(plain)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "decode Security Mode Complete", err)
		return nasreply.Handled()
	}

	if ue.SecurityContextIsValid() && integrityVerified {
		err := ue.UpdateSecurityContext()
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "update security context", err)
			return nasreply.Handled()
		}
	}

	if msg.IMEISV != nil {
		pei, err := imeiFromPEI(msg.IMEISV)
		if err != nil {
			// A malformed IMEISV yields no trusted equipment identity; reject and release
			// (the NAS guard has been stopped).
			amf.SendRegistrationReject(ctx, conn, amf.GmmCauseProtocolErrorUnspecified)
			ue.Deregister(ctx)

			return nasreply.Handled()
		}

		ue.Imei = pei
	}

	if msg.NASMessageContainer != nil {
		fgsRR, err := fgs.ParseRegistrationRequest(msg.NASMessageContainer)
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "decode NAS message container", err)
			return nasreply.Handled()
		}

		contextSetup(ctx, amfInstance, ue, fgsRR)

		return nasreply.Handled()
	}

	contextSetup(ctx, amfInstance, ue, conn.RegistrationRequest)

	return nasreply.Handled()
}

// imeiFromPEI decodes an IMEISV mobile-identity value into the shared equipment
// identity type (TS 24.501 §9.11.3.4).
func imeiFromPEI(imeisv []byte) (etsi.IMEI, error) {
	pei, err := fgs.PEIToString(imeisv)
	if err != nil {
		return etsi.IMEI{}, err
	}

	return etsi.NewIMEIFromPEI(pei)
}
