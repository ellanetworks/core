package gmm

import (
	ctxt "context"
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

func HandleAuthenticationFailure(ctx ctxt.Context, ue *context.AmfUe, authenticationFailure *nasMessage.AuthenticationFailure) error {
	logger.AmfLog.Debug("Handle Authentication Failure", zap.String("supi", ue.Supi))

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause5GMM := authenticationFailure.Cause5GMM.GetCauseValue()

	switch cause5GMM {
	case nasMessage.Cause5GMMMACFailure:
		ue.GmmLog.Warn("Authentication Failure Cause: Mac Failure")
		ue.State.Set(context.Deregistered)
		err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		ue.GmmLog.Warn("Authentication Failure Cause: Non-5G Authentication Unacceptable")
		ue.State.Set(context.Deregistered)
		err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	case nasMessage.Cause5GMMngKSIAlreadyInUse:
		ue.GmmLog.Warn("Authentication Failure Cause: NgKSI Already In Use")
		ue.AuthFailureCauseSynchFailureTimes = 0
		ue.GmmLog.Warn("Select new NgKsi")
		// select new ngksi
		if ue.NgKsi.Ksi < 6 { // ksi is range from 0 to 6
			ue.NgKsi.Ksi += 1
		} else {
			ue.NgKsi.Ksi = 0
		}

		err := gmm_message.SendAuthenticationRequest(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("send authentication request error: %s", err)
		}

		ue.GmmLog.Info("Sent authentication request")
	case nasMessage.Cause5GMMSynchFailure: // TS 24.501 5.4.1.3.7 case f
		ue.GmmLog.Warn("Authentication Failure 5GMM Cause: Synch Failure")

		ue.AuthFailureCauseSynchFailureTimes++
		if ue.AuthFailureCauseSynchFailureTimes >= 2 {
			ue.GmmLog.Warn("2 consecutive Synch Failure, terminate authentication procedure")
			ue.State.Set(context.Deregistered)
			err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return nil
		}

		auts := authenticationFailure.AuthenticationFailureParameter.GetAuthenticationFailureParameter()
		resynchronizationInfo := &models.ResynchronizationInfo{
			Auts: hex.EncodeToString(auts[:]),
		}

		response, err := consumer.SendUEAuthenticationAuthenticateRequest(ctx, ue, resynchronizationInfo)
		if err != nil {
			return fmt.Errorf("send UE Authentication Authenticate Request Error: %s", err.Error())
		}

		ue.AuthenticationCtx = response
		ue.ABBA = []uint8{0x00, 0x00}

		err = gmm_message.SendAuthenticationRequest(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("send authentication request error: %s", err)
		}

		ue.GmmLog.Info("Sent authentication request")
	}

	return nil
}

func handleAuthenticationFailure(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleAuthenticationFailure")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Authentication:
		err := HandleAuthenticationFailure(ctx, ue, msg.AuthenticationFailure)
		if err != nil {
			return fmt.Errorf("error handling authentication failure :%v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Authentication Failure message in state %s", ue.State.Current())
	}

	return nil
}
