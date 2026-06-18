// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func HandleInitialRegistration(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe) error {
	ue.ClearRegistrationData(ctx)

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	// update Kgnb/Kn3iwf
	err := ue.UpdateSecurityContext()
	if err != nil {
		return fmt.Errorf("error updating security context: %v", err)
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	subscriberProfile, err := amfInstance.GetSubscriberProfile(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("error getting subscriber profile: %v", err)
	}

	// Subscriber access control (Core Network type restriction, TS 23.501 §5.3.4):
	// if the profile does not permit 5G, reject with 5GMM cause #7 "5GS services
	// not allowed" (TS 24.501 §9.11.3.2).
	if !subscriberProfile.Allow5G {
		ranUe := ue.RanUe()
		if ranUe == nil {
			return fmt.Errorf("ue is not connected to RAN")
		}

		UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationReject).Inc()

		ue.Log.Info("registration rejected: 5G not allowed for subscriber")

		if err = message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMM5GSServicesNotAllowed); err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration Reject [5G not allowed for subscriber]")
	}

	if len(subscriberProfile.AllowedNssai) == 0 {
		ranUe := ue.RanUe()
		if ranUe == nil {
			return fmt.Errorf("ue is not connected to RAN")
		}

		UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationReject).Inc()

		err = message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration Reject [No allowed S-NSSAI in subscription]")
	}

	ue.Current().AllowedNssai = subscriberProfile.AllowedNssai
	ue.Current().Ambr = subscriberProfile.Ambr

	if conn.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", conn.RegistrationRequest.GetRAAI()))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.GetDRXValue()
		if drx > nasMessage.DRXcycleParameterT256 {
			ue.Log.Warn("UE requested reserved DRX value, treating as not specified", zap.Uint8("drxValue", drx))
			drx = nasMessage.DRXValueNotSpecified
		}

		ue.Current().UESpecificDRX = drx
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	ue.Log.Debug("use original GUTI", logger.GUTI(ue.Guti.String()))

	err = amfInstance.AddAmfUeToUePool(ue)
	if err != nil {
		return fmt.Errorf("error adding AMF UE to UE pool: %v", err)
	}

	ue.Current().T3502Value = amfInstance.T3502Value
	ue.Current().T3512Value = amfInstance.T3512Value

	err = amfInstance.ReAllocateGuti(ctx, ue, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationAccept).Inc()

	err = message.SendRegistrationAccept(ctx, amfInstance, ue, nil, nil, nil, nil, nil, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error sending GMM registration accept: %v", err)
	}

	return nil
}
