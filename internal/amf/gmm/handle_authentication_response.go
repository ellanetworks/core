package gmm

import (
	ctxt "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TS 24.501 5.4.1
func HandleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, authenticationResponse *nasMessage.AuthenticationResponse) error {
	logger.AmfLog.Debug("Handle Authentication Response", zap.String("supi", ue.Supi))

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.AuthenticationCtx == nil {
		return fmt.Errorf("ue Authentication Context is nil")
	}

	resStar := authenticationResponse.AuthenticationResponseParameter.GetRES()

	// Calculate HRES* (TS 33.501 Annex A.5)
	p0, err := hex.DecodeString(ue.AuthenticationCtx.Var5gAuthData.Rand)
	if err != nil {
		return err
	}

	p1 := resStar[:]
	concat := append(p0, p1...)
	hResStarBytes := sha256.Sum256(concat)
	hResStar := hex.EncodeToString(hResStarBytes[16:])

	if hResStar != ue.AuthenticationCtx.Var5gAuthData.HxresStar {
		ue.GmmLog.Error("HRES* Validation Failure", zap.String("received", hResStar), zap.String("expected", ue.AuthenticationCtx.Var5gAuthData.HxresStar))

		if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
			err := gmm_message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.GmmLog.Info("sent identity request")
			return nil
		} else {
			ue.State.Set(context.Deregistered)
			err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return nil
		}
	}

	response, err := consumer.SendAuth5gAkaConfirmRequest(ctx, ue, hex.EncodeToString(resStar[:]))
	if err != nil {
		return fmt.Errorf("authentication procedure failed: %s", err)
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
			err := gmm_message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.GmmLog.Info("sent identity request")
			return nil
		} else {
			err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
			if err != nil {
				// return fmt.Errorf("error sending GMM authentication reject: %v", err)
				logger.AmfLog.Error("error sending GMM authentication reject", zap.Error(err))
			}

			ue.State.Set(context.Deregistered)

			return nil
		}
	}

	return nil
}

func handleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleAuthenticationResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Authentication:
		err := HandleAuthenticationResponse(ctx, ue, msg.AuthenticationResponse)
		if err != nil {
			return fmt.Errorf("error handling authentication response: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Authentication Response message in state %s", ue.State.Current())
	}
	return nil
}
