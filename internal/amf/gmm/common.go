package gmm

import (
	"bytes"
	ctxt "context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

// TS 23.502 4.2.2.2.2 step 1
// If available, the last visited TAI shall be included in order to help the AMF produce Registration Area for the UE
func storeLastVisitedRegisteredTAI(ue *context.AmfUe, lastVisitedRegisteredTAI *nasType.LastVisitedRegisteredTAI) {
	if lastVisitedRegisteredTAI != nil {
		plmnID := nasConvert.PlmnIDToString(lastVisitedRegisteredTAI.Octet[1:4])
		nasTac := lastVisitedRegisteredTAI.GetTAC()
		tac := hex.EncodeToString(nasTac[:])

		tai := models.Tai{
			PlmnID: &models.PlmnID{
				Mcc: plmnID[:3],
				Mnc: plmnID[3:],
			},
			Tac: tac,
		}

		ue.LastVisitedRegisteredTai = tai
		ue.GmmLog.Debug("Ue Last Visited Registered Tai", zap.String("plmnID", plmnID), zap.String("tac", tac))
	}
}

// TS 23.502 4.2.2.2.3 Registration with AMF Re-allocation
func handleRequestedNssai(ctx ctxt.Context, ue *context.AmfUe, supportedPLMN *context.PlmnSupportItem) error {
	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.RequestedNSSAI != nil {
		requestedNssai, err := util.RequestedNssaiToModels(ue.RegistrationRequest.RequestedNSSAI)
		if err != nil {
			return fmt.Errorf("failed to decode requested NSSAI[%s]", err)
		}

		needSliceSelection := false
		var newAllowed *models.Snssai

		for _, requestedSnssai := range requestedNssai {
			if ue.InSubscribedNssai(requestedSnssai) {
				newAllowed = &models.Snssai{
					Sst: requestedSnssai.Sst,
					Sd:  requestedSnssai.Sd,
				}
			} else {
				needSliceSelection = true
				break
			}
		}

		ue.AllowedNssai = newAllowed

		if needSliceSelection {
			// Step 4
			ue.AllowedNssai = ue.SubscribedNssai

			var n1Message bytes.Buffer
			err = ue.RegistrationRequest.EncodeRegistrationRequest(&n1Message)
			if err != nil {
				return fmt.Errorf("failed to encode registration request: %s", err)
			}
			return nil
		}
	}

	// if registration request has no requested nssai, or non of snssai in requested nssai is permitted by nssf
	// then use ue subscribed snssai which is marked as default as allowed nssai
	if ue.AllowedNssai == nil {
		var newAllowed *models.Snssai
		if amfSelf.InPlmnSupport(ctx, *ue.SubscribedNssai, supportedPLMN) {
			newAllowed = ue.SubscribedNssai
		}
		ue.AllowedNssai = newAllowed
	}
	return nil
}

func plmnIDStringToModels(plmnIDStr string) models.PlmnID {
	var plmnID models.PlmnID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}

func negotiateDRXParameters(ue *context.AmfUe, requestedDRXParameters *nasType.RequestedDRXParameters) {
	if requestedDRXParameters != nil {
		switch requestedDRXParameters.GetDRXValue() {
		case nasMessage.DRXcycleParameterT32:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT32
		case nasMessage.DRXcycleParameterT64:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT64
		case nasMessage.DRXcycleParameterT128:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT128
		case nasMessage.DRXcycleParameterT256:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT256
		case nasMessage.DRXValueNotSpecified:
			fallthrough
		default:
			ue.UESpecificDRX = nasMessage.DRXValueNotSpecified
		}
	}
}

func getAndSetSubscriberData(ctx ctxt.Context, ue *context.AmfUe) error {
	bitRate, dnn, err := context.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate
	ue.SubscriptionDataValid = true

	return nil
}
