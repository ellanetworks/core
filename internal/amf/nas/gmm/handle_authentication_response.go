package gmm

import (
	ctxt "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TS 24.501 5.4.1
func handleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Authentication Response", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleAuthenticationResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != context.Authentication {
		return fmt.Errorf("state mismatch: receive Authentication Response message in state %s", ue.State.Current())
	}

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.AuthenticationCtx == nil {
		return fmt.Errorf("ue Authentication Context is nil")
	}

	resStar := msg.AuthenticationResponse.AuthenticationResponseParameter.GetRES()

	// Calculate HRES* (TS 33.501 Annex A.5)
	p0, err := hex.DecodeString(ue.AuthenticationCtx.Rand)
	if err != nil {
		return fmt.Errorf("failed to decode RAND: %s", err)
	}

	p1 := resStar[:]
	concat := append(p0, p1...)
	hResStarBytes := sha256.Sum256(concat)
	hResStar := hex.EncodeToString(hResStarBytes[16:])

	if hResStar != ue.AuthenticationCtx.HxresStar {
		ue.Log.Error("HRES* Validation Failure", zap.String("received", hResStar), zap.String("expected", ue.AuthenticationCtx.HxresStar))

		if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
			err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.Log.Info("sent identity request")
			return nil
		}

		ue.State.Set(context.Deregistered)
		err := message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	}

	response, err := ausf.Auth5gAkaComfirmRequestProcedure(ctx, hex.EncodeToString(resStar[:]), ue.Suci)
	if err != nil {
		return fmt.Errorf("ausf 5G-AKA Confirm Request failed: %s", err.Error())
	}

	switch response.AuthResult {
	case models.AuthResultSuccess:
		ue.Kseaf = response.Kseaf
		ue.Supi = response.Supi
		ue.DerivateKamf()
		ue.State.Set(context.SecurityMode)

		return securityMode(ctx, ue)

	case models.AuthResultFailure:
		if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
			err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.Log.Info("sent identity request")
			return nil
		}

		ue.State.Set(context.Deregistered)
		err := message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	}
	return nil
}
