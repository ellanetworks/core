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

	// update Kgnb/Kn3iwf
	err := ue.UpdateSecurityContext()
	if err != nil {
		return fmt.Errorf("error updating security context: %v", err)
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	ue.AllowedNssai = operatorInfo.SupportedPLMN.SNssai

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", ue.RegistrationRequest.GetRAAI()))
	}

	if ue.RegistrationRequest.RequestedDRXParameters != nil {
		drx := ue.RegistrationRequest.GetDRXValue()
		if drx > nasMessage.DRXcycleParameterT256 {
			ue.Log.Warn("UE requested reserved DRX value, treating as not specified", zap.Uint8("drxValue", drx))
			drx = nasMessage.DRXValueNotSpecified
		}

		ue.UESpecificDRX = drx
	}

	bitRate, err := amfInstance.GetSubscriberBitrate(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Ambr = bitRate

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	ue.Log.Debug("use original GUTI", logger.GUTI(ue.Guti.String()))

	err = amfInstance.AddAmfUeToUePool(ue)
	if err != nil {
		return fmt.Errorf("error adding AMF UE to UE pool: %v", err)
	}

	ue.T3502Value = amfInstance.T3502Value
	ue.T3512Value = amfInstance.T3512Value

	err = amfInstance.ReAllocateGuti(ctx, ue, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(ue.RegistrationType5GS), RegistrationAccept).Inc()

	err = message.SendRegistrationAccept(ctx, amfInstance, ue, nil, nil, nil, nil, nil, operatorInfo.SupportedPLMN, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error sending GMM registration accept: %v", err)
	}

	return nil
}
