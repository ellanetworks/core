package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleInitialRegistration(ctx ctxt.Context, ue *context.AmfUe) error {
	amfSelf := context.AMFSelf()

	ue.ClearRegistrationData()

	// update Kgnb/Kn3iwf
	ue.UpdateSecurityContext()

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if ue.SubscribedNssai == nil {
		ue.SubscribedNssai = operatorInfo.SupportedPLMN.SNssai
	}

	if err := handleRequestedNssai(ctx, ue, operatorInfo.SupportedPLMN); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	}

	if ue.AllowedNssai == nil {
		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			ue.GmmLog.Error("error sending registration reject", zap.Error(err))
		}
		err = ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			ue.GmmLog.Error("error sending ue context release command", zap.Error(err))
		}
		ue.Remove()
		return fmt.Errorf("no allowed nssai")
	}

	storeLastVisitedRegisteredTAI(ue, ue.RegistrationRequest.LastVisitedRegisteredTAI)

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.GmmLog.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", ue.RegistrationRequest.MICOIndication.GetRAAI()))
	}

	negotiateDRXParameters(ue, ue.RegistrationRequest.RequestedDRXParameters)

	if ue.ServingAmfChanged ||
		!ue.SubscriptionDataValid {
		if err := getAndSetSubscriberData(ctx, ue); err != nil {
			return err
		}
	}

	if !context.SubscriberExists(ctx, ue.Supi) {
		ue.GmmLog.Error("Subscriber does not exist", zap.Error(err))
		err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return fmt.Errorf("ue not found in database: %s", ue.Supi)
	}

	amfSelf.AllocateRegistrationArea(ctx, ue, operatorInfo.Tais)
	ue.GmmLog.Debug("use original GUTI", zap.String("guti", ue.Guti))

	amfSelf.AddAmfUeToUePool(ue, ue.Supi)
	ue.T3502Value = amfSelf.T3502Value
	ue.T3512Value = amfSelf.T3512Value

	amfSelf.ReAllocateGutiToUe(ctx, ue, operatorInfo.Guami)
	// check in specs if we need to wait for confirmation before freeing old GUTI
	amfSelf.FreeOldGuti(ue)

	err = message.SendRegistrationAccept(ctx, ue, nil, nil, nil, nil, nil, operatorInfo.SupportedPLMN, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error sending GMM registration accept: %v", err)
	}

	return nil
}
