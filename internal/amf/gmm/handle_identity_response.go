package gmm

import (
	ctxt "context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func HandleIdentityResponse(ue *context.AmfUe, identityResponse *nasMessage.IdentityResponse) error {
	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	mobileIdentityContents := identityResponse.MobileIdentity.GetMobileIdentityContents()
	if len(mobileIdentityContents) == 0 {
		return fmt.Errorf("mobile identity is empty")
	}

	switch nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) {
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnID string
		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentityContents)
		ue.PlmnID = PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		_, guti := nasConvert.GutiToString(mobileIdentityContents)
		ue.Guti = guti
		ue.GmmLog.Debug("get GUTI", zap.String("guti", guti))
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		sTmsi := hex.EncodeToString(mobileIdentityContents[1:])
		if tmp, err := strconv.ParseInt(sTmsi[4:], 10, 32); err != nil {
			return err
		} else {
			ue.Tmsi = int32(tmp)
		}
		ue.GmmLog.Debug("get 5G-S-TMSI", zap.String("5G-S-TMSI", sTmsi))
	case nasMessage.MobileIdentity5GSTypeImei:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		imei := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imei
		ue.GmmLog.Debug("get PEI", zap.String("PEI", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		imeisv := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imeisv
		ue.GmmLog.Debug("get PEI", zap.String("PEI", imeisv))
	}
	return nil
}

func handleIdentityResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleIdentityResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Authentication:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}

		pass, err := AuthenticationProcedure(ctx, ue)
		if err != nil {
			ue.State.Set(context.Deregistered)
			return fmt.Errorf("error in authentication procedure: %v", err)
		}
		if pass {
			ue.State.Set(context.SecurityMode)
			return securityMode(ctx, ue)
		}
		ue.State.Set(context.Authentication)
		return nil

	case context.ContextSetup:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}
		switch ue.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			if err := HandleInitialRegistration(ctx, ue); err != nil {
				ue.State.Set(context.Deregistered)
				return fmt.Errorf("error handling initial registration: %v", err)
			}
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, ue); err != nil {
				ue.State.Set(context.Deregistered)
				return fmt.Errorf("error handling mobility and periodic registration updating: %v", err)
			}
		}
	default:
		return fmt.Errorf("state mismatch: receive Identity Response message in state %s", ue.State.Current())
	}
	return nil
}
