package gmm

import (
	"context"
	"errors"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
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

// Registration result labels
const (
	RegistrationAccept = "accept"
	RegistrationReject = "reject"
)

// Handle cleartext IEs of Registration Request, which cleattext IEs defined in TS 24.501 4.4.6
func handleRegistrationRequestMessage(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, registrationRequest *nasMessage.RegistrationRequest) error {
	if ue.RanUe == nil {
		return fmt.Errorf("RanUe is nil")
	}

	// MacFailed is set if plain Registration Request message received with GUTI/SUCI or
	// integrity protected Registration Reguest message received but mac verification Failed
	if ue.MacFailed {
		ue.SecurityContextAvailable = false
	}

	ue.SetOnGoing(amfContext.OnGoingProcedureRegistration)

	if ue.T3513 != nil {
		ue.T3513.Stop()
		ue.T3513 = nil // clear the timer
	}

	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	// TS 24.501 4.4.6: If NASMessageContainer is present, it contains a ciphered inner Registration Request
	// carrying non-cleartext IEs, which must be decrypted and processed instead of the outer message.
	if registrationRequest.NASMessageContainer != nil {
		contents := registrationRequest.GetNASMessageContainerContents()

		err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionUplink, contents)
		if err != nil {
			UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(ue.RegistrationType5GS), RegistrationReject).Inc()

			err1 := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err1 != nil {
				return fmt.Errorf("error sending registration reject after error decrypting: %v", err1)
			}

			return fmt.Errorf("failed to decrypt NAS message - sent registration reject: %v", err)
		}

		m := nas.NewMessage()

		if err := m.GmmMessageDecode(&contents); err != nil {
			UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(ue.RegistrationType5GS), RegistrationReject).Inc()

			err1 := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err1 != nil {
				return fmt.Errorf("error sending registration reject after error decoding: %v", err1)
			}

			return fmt.Errorf("failed to decode NAS message - sent registration reject: %v", err)
		}

		messageType := m.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest {
			return fmt.Errorf("expected registration request, got %d", messageType)
		}

		registrationRequest = m.RegistrationRequest

		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	ue.RegistrationRequest = registrationRequest
	ue.RegistrationType5GS = registrationRequest.GetRegistrationType5GS()

	regName := getRegistrationType5GSName(ue.RegistrationType5GS)

	ue.Log.Debug("Received Registration Request", zap.String("registrationType", regName))

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSReserved {
		ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	}

	mobileIdentity5GSContents := registrationRequest.GetMobileIdentity5GSContents()
	if len(mobileIdentity5GSContents) == 0 {
		return errors.New("mobile identity 5GS is empty")
	}

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	ue.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch ue.IdentityTypeUsedForRegistration { // get type of identity
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.Log.Debug("No Identity used for registration")
	case nasMessage.MobileIdentity5GSTypeSuci:
		ue.Log.Debug("UE used SUCI identity for registration")

		var plmnID string

		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = plmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		_, guti := nasConvert.GutiToString(mobileIdentity5GSContents)
		ue.Guti = guti
		ue.Log.Debug("UE used GUTI identity for registration", zap.String("guti", guti))
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imei
		ue.Log.Debug("UE used IMEI identity for registration", zap.String("imei", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imeisv
		ue.Log.Debug("UE used IMEISV identity for registration", zap.String("imeisv", imeisv))
	}

	// NgKsi: TS 24.501 9.11.3.32
	switch registrationRequest.GetTSC() {
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
	if !amfContext.InTaiList(ue.Tai, operatorInfo.Tais) {
		UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(ue.RegistrationType5GS), RegistrationReject).Inc()

		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMTrackingAreaNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration Reject [Tracking area not allowed]")
	}

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSInitialRegistration && registrationRequest.UESecurityCapability == nil {
		UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(ue.RegistrationType5GS), RegistrationReject).Inc()

		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration request does not contain UE security capability for initial registration")
	}

	if registrationRequest.UESecurityCapability != nil {
		ue.UESecurityCapability = registrationRequest.UESecurityCapability
	}

	return nil
}

func handleRegistrationRequest(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State {
	case amfContext.Deregistered, amfContext.Registered:
		if err := handleRegistrationRequestMessage(ctx, amf, ue, msg.RegistrationRequest); err != nil {
			return fmt.Errorf("failed handling registration request: %v", err)
		}

		ue.State = amfContext.Authentication

		pass, err := authenticationProcedure(ctx, amf, ue)
		if err != nil {
			ue.State = amfContext.Deregistered

			UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(ue.RegistrationType5GS), RegistrationReject).Inc()

			err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
			if err != nil {
				return fmt.Errorf("error sending registration reject: %v", err)
			}

			return nil
		}

		if pass {
			return securityMode(ctx, amf, ue)
		}

	case amfContext.SecurityMode:
		ue.SecurityContextAvailable = false
		ue.T3560.Stop()
		ue.T3560 = nil
		ue.State = amfContext.Deregistered

		return HandleGmmMessage(ctx, amf, ue, msg)
	case amfContext.ContextSetup:
		ue.State = amfContext.Deregistered
		ue.Log.Info("state reset to Deregistered")

		return nil
	default:
		return fmt.Errorf("state mismatch: receive Registration Request message in state %s", ue.State)
	}

	return nil
}
