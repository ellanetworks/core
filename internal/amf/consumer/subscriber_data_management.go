package consumer

import (
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/udm/producer"
)

func SDMGetAmData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	data, err := producer.GetAmData(ue.Supi)
	if err != nil {
		return nil, err
	}
	ue.AccessAndMobilitySubscriptionData = data
	ue.Gpsi = data.Gpsis[0]
	return nil, nil
}

func SDMGetSmfSelectData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	data, err := producer.GetSmfSelectData(ue.Supi)
	if err != nil {
		return nil, err
	}
	ue.SmfSelectionData = data
	return nil, nil
}

func SDMGetUeContextInSmfData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	data, err := producer.GetUeContextInSmfData(ue.Supi)
	if err != nil {
		return nil, err
	}
	ue.UeContextInSmfData = data
	return nil, nil
}

func SDMSubscribe(ue *context.AmfUe) (*models.ProblemDetails, error) {
	amfSelf := context.AMF_Self()
	sdmSubscription := &models.SdmSubscription{
		NfInstanceId: amfSelf.NfId,
		PlmnId:       &ue.PlmnId,
	}

	err := producer.CreateSubscription(sdmSubscription, ue.Supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problemDetails, nil
	}
	return nil, nil
}

func SDMGetSliceSelectionSubscriptionData(ue *context.AmfUe) (problemDetails *models.ProblemDetails, err error) {
	nssai, err := producer.GetNssai(ue.Supi)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problemDetails, nil
	}
	for _, defaultSnssai := range nssai.DefaultSingleNssais {
		subscribedSnssai := models.SubscribedSnssai{
			SubscribedSnssai: &models.Snssai{
				Sst: defaultSnssai.Sst,
				Sd:  defaultSnssai.Sd,
			},
			DefaultIndication: true,
		}
		ue.SubscribedNssai = append(ue.SubscribedNssai, subscribedSnssai)
	}
	for _, snssai := range nssai.SingleNssais {
		subscribedSnssai := models.SubscribedSnssai{
			SubscribedSnssai: &models.Snssai{
				Sst: snssai.Sst,
				Sd:  snssai.Sd,
			},
			DefaultIndication: false,
		}
		ue.SubscribedNssai = append(ue.SubscribedNssai, subscribedSnssai)
	}
	return nil, nil
}
