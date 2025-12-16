package gmm

import (
	ctxt "context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasConvert"
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
		ue.Log.Debug("Ue Last Visited Registered Tai", zap.String("plmnID", plmnID), zap.String("tac", tac))
	}
}

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
