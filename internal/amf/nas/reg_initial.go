// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func HandleInitialRegistration(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) error {
	ue.ClearRegistrationData(ctx)

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	err := ue.UpdateSecurityContext()
	if err != nil {
		return fmt.Errorf("error updating security context: %v", err)
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	subscriberProfile, err := amfInstance.GetSubscriberProfile(ctx, ue.SupiValue())
	if err != nil {
		return fmt.Errorf("error getting subscriber profile: %v", err)
	}

	// Subscriber access control (Core Network type restriction, TS 23.501):
	// if the profile does not permit 5G, reject with 5GMM cause #7 "5GS services
	// not allowed" (TS 24.501).
	if !subscriberProfile.Allow5G {
		ranUe := ue.RanUe()
		if ranUe == nil {
			return fmt.Errorf("ue is not connected to RAN")
		}

		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		ue.Log.Info("registration rejected: 5G not allowed for subscriber")

		amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMM5GSServicesNotAllowed)

		return fmt.Errorf("registration Reject [5G not allowed for subscriber]")
	}

	if len(subscriberProfile.AllowedNssai) == 0 {
		ranUe := ue.RanUe()
		if ranUe == nil {
			return fmt.Errorf("ue is not connected to RAN")
		}

		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMM5GSServicesNotAllowed)

		return fmt.Errorf("registration Reject [No allowed S-NSSAI in subscription]")
	}

	ue.AllowedNssai = subscriberProfile.AllowedNssai
	ue.Ambr = subscriberProfile.Ambr

	if conn.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", conn.RegistrationRequest.GetRAAI()))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.GetDRXValue()
		if drx > nasMessage.DRXcycleParameterT256 {
			ue.Log.Warn("UE requested reserved DRX value, treating as not specified", zap.Uint8("drxValue", drx))
			drx = nasMessage.DRXValueNotSpecified
		}

		ue.UESpecificDRX = drx
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	guti := ue.Guti()
	ue.Log.Debug("use original GUTI", logger.GUTI(guti.String()))

	// TS 24.501: a successful initial registration supersedes any
	// earlier 5GMM context for this subscriber. The old context is deleted only
	// here, once the new registration is authenticated, so that an
	// unauthenticated registration on a fresh context never tears it down.
	if existing, ok := amfInstance.FindUeContextBySupi(ue.SupiValue()); ok && existing != ue {
		amfInstance.DeregisterAndRemoveUeContext(ctx, existing)
	}

	err = amfInstance.AddUeContextToPool(ue)
	if err != nil {
		return fmt.Errorf("error adding amf.AMF UE to UE pool: %v", err)
	}

	ue.T3502Value = amfInstance.T3502Value
	ue.T3512Value = amfInstance.T3512Value

	err = amfInstance.ReAllocateGuti(ctx, ue, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

	amf.SendRegistrationAccept(ctx, amfInstance, ue, nil, nil, nil, nil, nil, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

	return nil
}
