package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
)

func inSubscribedNssai(subscribedSnssai *models.Snssai, targetSNssai *models.Snssai) bool {
	return subscribedSnssai.Sst == targetSNssai.Sst && subscribedSnssai.Sd == targetSNssai.Sd
}

// TS 23.502 4.2.2.2.3 Registration with AMF Re-allocation
func handleRequestedNssai(ue *context.AmfUe, subscribedSssai *models.Snssai) error {
	if ue.RegistrationRequest.RequestedNSSAI != nil {
		requestedNssai, err := util.RequestedNssaiToModels(ue.RegistrationRequest.RequestedNSSAI)
		if err != nil {
			return fmt.Errorf("failed to decode requested NSSAI[%s]", err)
		}

		needSliceSelection := false
		var newAllowed *models.Snssai

		for _, requestedSnssai := range requestedNssai {
			if inSubscribedNssai(subscribedSssai, requestedSnssai) {
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
			ue.AllowedNssai = subscribedSssai
			return nil
		}
	}

	// if registration request has no requested nssai, or non of snssai in requested nssai is permitted by nssf
	// then use ue subscribed snssai which is marked as default as allowed nssai
	if ue.AllowedNssai == nil {
		ue.AllowedNssai = subscribedSssai
	}
	return nil
}

func plmnIDStringToModels(plmnIDStr string) models.PlmnID {
	var plmnID models.PlmnID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}

func getAndSetSubscriberData(ctx ctxt.Context, ue *context.AmfUe) error {
	bitRate, dnn, err := context.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate

	return nil
}
