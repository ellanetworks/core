package gmm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TS 24.501 5.4.1
func handleAuthenticationResponse(ctx context.Context, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Authentication Response", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleAuthenticationResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != amfContext.Authentication {
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

		ue.State.Set(amfContext.Deregistered)
		err := message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	}

	supi, kseaf, err := ausf.Auth5gAkaComfirmRequestProcedure(hex.EncodeToString(resStar[:]), ue.Suci)
	if err != nil {
		logger.AmfLog.Error("5G AKA Confirmation Request Procedure failed", zap.Error(err))

		if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
			err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.Log.Info("sent identity request")
			return nil
		}

		ue.State.Set(amfContext.Deregistered)
		err := message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	}

	ue.Kseaf = kseaf
	ue.Supi = supi

	ue.DerivateKamf()

	ue.State.Set(amfContext.SecurityMode)

	return securityMode(ctx, ue)
}
