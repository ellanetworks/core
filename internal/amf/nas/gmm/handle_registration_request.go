package gmm

import (
	ctxt "context"
	"errors"
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func getRegistrationType5GSName(regType5Gs uint8) string {
	switch regType5Gs {
	case nasMessage.RegistrationType5GSInitialRegistration:
		return "Initial Registration"
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		return "Mobility Registration Updating"
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		return "Periodic Registration Updating"
	case nasMessage.RegistrationType5GSEmergencyRegistration:
		return "Emergency Registration"
	case nasMessage.RegistrationType5GSReserved:
		return "Reserved"
	default:
		return "Unknown"
	}
}

// Handle cleartext IEs of Registration Request, which cleattext IEs defined in TS 24.501 4.4.6
func HandleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, registrationRequest *nasMessage.RegistrationRequest) error {
	var guamiFromUeGuti models.Guami

	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	if ue.RanUe == nil {
		return fmt.Errorf("RanUe is nil")
	}

	// MacFailed is set if plain Registration Request message received with GUTI/SUCI or
	// integrity protected Registration Reguest message received but mac verification Failed
	if ue.MacFailed {
		ue.SecurityContextAvailable = false
	}

	ue.SetOnGoing(&context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedureRegistration,
	})

	if ue.T3513 != nil {
		ue.T3513.Stop()
		ue.T3513 = nil // clear the timer
	}
	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	ue.RegistrationRequest = registrationRequest
	ue.RegistrationType5GS = registrationRequest.NgksiAndRegistrationType5GS.GetRegistrationType5GS()
	regName := getRegistrationType5GSName(ue.RegistrationType5GS)
	ue.GmmLog.Debug("Received Registration Request", zap.String("registrationType", regName))

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSReserved {
		ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	}

	mobileIdentity5GSContents := registrationRequest.MobileIdentity5GS.GetMobileIdentity5GSContents()
	if len(mobileIdentity5GSContents) == 0 {
		return errors.New("mobile identity 5GS is empty")
	}

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	ue.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch ue.IdentityTypeUsedForRegistration { // get type of identity
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.GmmLog.Debug("No Identity used for registration")
	case nasMessage.MobileIdentity5GSTypeSuci:
		ue.GmmLog.Debug("UE used SUCI identity for registration")
		var plmnID string
		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = plmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		guamiFromUeGutiTmp, guti := util.GutiToString(mobileIdentity5GSContents)
		guamiFromUeGuti = guamiFromUeGutiTmp
		ue.Guti = guti
		ue.GmmLog.Debug("UE used GUTI identity for registration", zap.String("guti", guti))

		if reflect.DeepEqual(guamiFromUeGuti, operatorInfo.Guami) {
			ue.ServingAmfChanged = false
		} else {
			ue.GmmLog.Debug("Serving AMF has changed but 5G-Core is not supporting for now")
			ue.ServingAmfChanged = false
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imei
		ue.GmmLog.Debug("UE used IMEI identity for registration", zap.String("imei", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imeisv
		ue.GmmLog.Debug("UE used IMEISV identity for registration", zap.String("imeisv", imeisv))
	}

	// NgKsi: TS 24.501 9.11.3.32
	switch registrationRequest.NgksiAndRegistrationType5GS.GetTSC() {
	case nasMessage.TypeOfSecurityContextFlagNative:
		ue.NgKsi.Tsc = models.ScTypeNative
	case nasMessage.TypeOfSecurityContextFlagMapped:
		ue.NgKsi.Tsc = models.ScTypeMapped
	}
	ue.NgKsi.Ksi = int32(registrationRequest.NgksiAndRegistrationType5GS.GetNasKeySetIdentifiler())
	if ue.NgKsi.Tsc == models.ScTypeNative && ue.NgKsi.Ksi != 7 {
	} else {
		ue.NgKsi.Tsc = models.ScTypeNative
		ue.NgKsi.Ksi = 0
	}

	// Copy UserLocation from ranUe
	ue.Location = ue.RanUe.Location
	ue.Tai = ue.RanUe.Tai

	// Check TAI
	taiList := make([]models.Tai, len(operatorInfo.Tais))
	copy(taiList, operatorInfo.Tais)
	for i := range taiList {
		tac, err := util.TACConfigToModels(taiList[i].Tac)
		if err != nil {
			logger.AmfLog.Warn("failed to convert TAC to models.Tac", zap.Error(err), zap.String("tac", taiList[i].Tac))
			continue
		}
		taiList[i].Tac = tac
	}
	if !context.InTaiList(ue.Tai, taiList) {
		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMTrackingAreaNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSInitialRegistration && registrationRequest.UESecurityCapability == nil {
		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration request does not contain UE security capability for initial registration")
	}

	if registrationRequest.UESecurityCapability != nil {
		ue.UESecurityCapability = registrationRequest.UESecurityCapability
	}

	if ue.ServingAmfChanged {
		logger.AmfLog.Debug("Serving AMF has changed - Unsupported")
	}

	return nil
}

func handleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Registration Request", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleRegistrationRequest")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleRegistrationRequest(ctx, ue, msg.RegistrationRequest); err != nil {
			return fmt.Errorf("failed handling registration request")
		}

		pass, err := AuthenticationProcedure(ctx, ue)
		if err != nil {
			ue.State.Set(context.Deregistered)
			err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err != nil {
				return fmt.Errorf("error sending registration reject: %v", err)
			}
			return nil
		}
		if pass {
			ue.State.Set(context.SecurityMode)
			return securityMode(ctx, ue)
		}

		ue.State.Set(context.Authentication)

	case context.SecurityMode:
		ue.SecurityContextAvailable = false
		ue.T3560.Stop()
		ue.T3560 = nil
		ue.State.Set(context.Deregistered)

		return HandleGmmMessage(ctx, ue, msg)
	case context.ContextSetup:
		ue.State.Set(context.Deregistered)
		ue.GmmLog.Info("state reset to Deregistered")
		return nil
	}

	return nil
}
