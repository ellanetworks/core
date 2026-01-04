package gmm

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
)

func updateUEIdentity(ue *amfContext.AmfUe, mobileIdentityContents []uint8) error {
	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	if len(mobileIdentityContents) == 0 {
		return fmt.Errorf("mobile identity is empty")
	}

	switch nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) {
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnID string

		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentityContents)
		ue.PlmnID = plmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}

		_, guti := nasConvert.GutiToString(mobileIdentityContents)
		ue.Guti = guti
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}

		sTmsi := hex.EncodeToString(mobileIdentityContents[1:])

		tmp, err := strconv.ParseInt(sTmsi[4:], 10, 32)
		if err != nil {
			return fmt.Errorf("could not parse 5G-S-TMSI: %v", err)
		}

		ue.Tmsi = int32(tmp)
	case nasMessage.MobileIdentity5GSTypeImei:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}

		imei := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imei
	case nasMessage.MobileIdentity5GSTypeImeisv:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}

		imeisv := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imeisv
	}

	return nil
}

func handleIdentityResponse(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, msg *nasMessage.IdentityResponse) error {
	switch ue.State {
	case amfContext.Authentication:
		mobileIdentityContents := msg.GetMobileIdentityContents()

		if err := updateUEIdentity(ue, mobileIdentityContents); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}

		ue.State = amfContext.Authentication

		pass, err := authenticationProcedure(ctx, amf, ue)
		if err != nil {
			ue.State = amfContext.Deregistered
			return fmt.Errorf("error in authentication procedure: %v", err)
		}

		if pass {
			return securityMode(ctx, amf, ue)
		}

		return nil

	case amfContext.ContextSetup:
		mobileIdentityContents := msg.GetMobileIdentityContents()

		if err := updateUEIdentity(ue, mobileIdentityContents); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}

		switch ue.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			if err := HandleInitialRegistration(ctx, amf, ue); err != nil {
				ue.State = amfContext.Deregistered
				return fmt.Errorf("error handling initial registration: %v", err)
			}
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amf, ue); err != nil {
				ue.State = amfContext.Deregistered
				return fmt.Errorf("error handling mobility and periodic registration updating: %v", err)
			}
		}
	default:
		return fmt.Errorf("state mismatch: receive Identity Response message in state %s", ue.State)
	}

	return nil
}
