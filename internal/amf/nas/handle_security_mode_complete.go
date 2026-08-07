// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// TS 33.501
func handleSecurityModeComplete(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *fgs.SecurityModeComplete, integrityVerified bool) nasreply.Disposition {
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

	ue.EndKeyChainProc(procedure.SecurityMode)

	if ue.SecurityContextIsValid() && integrityVerified {
		err := ue.UpdateSecurityContext()
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "update security context", err)
			return nasreply.Handled()
		}
	}

	if msg.IMEISV != nil {
		pei, err := imeiFromPEI(*msg.IMEISV)
		if err != nil {
			// A malformed IMEISV yields no trusted equipment identity; reject and release
			// (the NAS guard has been stopped).
			amf.SendRegistrationReject(ctx, conn, fgs.GMMCauseProtocolErrorUnspecified)
			ue.Deregister(ctx)

			return nasreply.Handled()
		}

		ue.Imei = pei
	}

	if msg.NASMessageContainer != nil {
		fgsRR, err := fgs.ParseRegistrationRequest(msg.NASMessageContainer)
		if !decoded(ctx, "RegistrationRequest", err) {
			abortRegistration(ctx, amfInstance, ue, "decode NAS message container", err)
			return nasreply.Handled()
		}

		// The container carries the complete message; it becomes the oracle a
		// later retransmission is compared against, cloned because the message
		// keeps a reference to it (TS 24.501 §4.4.6).
		contextSetup(ctx, amfInstance, ue, fgsRR, slices.Clone(msg.NASMessageContainer))

		return nasreply.Handled()
	}

	// TS 24.501 §4.4.6 case a: a UE whose REGISTRATION REQUEST was cleartext-only
	// repeats it here, so its absence leaves the non-cleartext IEs unauthenticated.
	if !conn.RegistrationRequestProtected {
		abortRegistration(ctx, amfInstance, ue, "NAS message container", errNoRegistrationContainer)

		return nasreply.Handled()
	}

	contextSetup(ctx, amfInstance, ue, conn.RegistrationRequest, conn.RegistrationRequestPlain)

	return nasreply.Handled()
}

// errNoRegistrationContainer reports a SECURITY MODE COMPLETE that omits the
// REGISTRATION REQUEST a cleartext-only initial message has to repeat.
var errNoRegistrationContainer = errors.New("SECURITY MODE COMPLETE carries no NAS message container")

// imeiFromPEI renders a decoded IMEISV mobile identity as the shared equipment
// identity type (TS 24.501 §9.11.3.4).
func imeiFromPEI(id fgs.MobileIdentity) (etsi.IMEI, error) {
	if id.PEI == nil || !id.PEI.Valid() {
		return etsi.IMEI{}, fmt.Errorf("mobile identity %s is not a well-formed equipment identity", id.Type())
	}

	return etsi.NewIMEIFromPEI(id.PEI.String())
}
