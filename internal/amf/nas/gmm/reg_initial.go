package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func HandleInitialRegistration(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe) error {
	ue.ClearRegistrationData()

	// update Kgnb/Kn3iwf
	err := ue.UpdateSecurityContext()
	if err != nil {
		return fmt.Errorf("error updating security context: %v", err)
	}

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	ue.AllowedNssai = operatorInfo.SupportedPLMN.SNssai

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", ue.RegistrationRequest.MICOIndication.GetRAAI()))
	}

	if ue.RegistrationRequest.RequestedDRXParameters != nil {
		ue.UESpecificDRX = ue.RegistrationRequest.RequestedDRXParameters.GetDRXValue()
	}

	bitRate, dnn, err := amf.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate

	if !amf.SubscriberExists(ctx, ue.Supi) {
		ue.Log.Error("Subscriber does not exist", zap.Error(err))

		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		ue.Log.Info("sent registration reject to UE")

		return fmt.Errorf("ue not found in database: %s", ue.Supi)
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	ue.Log.Debug("use original GUTI", zap.String("guti", ue.Guti))

	err = amf.AddAmfUeToUePool(ue)
	if err != nil {
		return fmt.Errorf("error adding AMF UE to UE pool: %v", err)
	}

	ue.T3502Value = amf.T3502Value
	ue.T3512Value = amf.T3512Value

	err = ue.ReAllocateGuti(operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	// check in specs if we need to wait for confirmation before freeing old GUTI
	ue.FreeOldGuti()

	err = message.SendRegistrationAccept(ctx, amf, ue, nil, nil, nil, nil, nil, operatorInfo.SupportedPLMN, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error sending GMM registration accept: %v", err)
	}

	return nil
}
