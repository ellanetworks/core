package consumer

import (
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/nssf/producer"
)

func NSSelectionGetForRegistration(ue *context.AmfUe, requestedNssai []models.MappingOfSnssai) (
	*models.ProblemDetails, error,
) {
	amfSelf := context.AMF_Self()
	sliceInfo := models.SliceInfoForRegistration{
		SubscribedNssai: ue.SubscribedNssai,
	}

	for _, snssai := range requestedNssai {
		sliceInfo.RequestedNssai = append(sliceInfo.RequestedNssai, *snssai.ServingSnssai)
		if snssai.HomeSnssai != nil {
			sliceInfo.MappingOfNssai = append(sliceInfo.MappingOfNssai, snssai)
		}
	}

	amfType := models.NfType_AMF
	params := producer.NsselectionQueryParameter{
		NfType:                          &amfType,
		NfId:                            amfSelf.NfId,
		SliceInfoRequestForRegistration: &sliceInfo,
	}

	res, err := producer.GetNSSelection(params)
	if err != nil {
		logger.ConsumerLog.Warnf("GetNSSelection failed: %+v", err)
		return nil, err
	}
	ue.NetworkSliceInfo = res
	for _, allowedNssai := range res.AllowedNssaiList {
		ue.AllowedNssai[allowedNssai.AccessType] = allowedNssai.AllowedSnssaiList
	}
	ue.ConfiguredNssai = res.ConfiguredNssai
	return nil, nil
}

func NSSelectionGetForPduSession(ue *context.AmfUe, snssai models.Snssai) (
	*models.AuthorizedNetworkSliceInfo, *models.ProblemDetails, error,
) {
	amfSelf := context.AMF_Self()
	sliceInfoForPduSession := models.SliceInfoForPduSession{
		SNssai:            &snssai,
		RoamingIndication: models.RoamingIndication_NON_ROAMING,
	}

	amfType := models.NfType_AMF
	params := producer.NsselectionQueryParameter{
		NfType:                        &amfType,
		NfId:                          amfSelf.NfId,
		SliceInfoRequestForPduSession: &sliceInfoForPduSession,
	}

	res, err := producer.GetNSSelection(params)
	if err != nil {
		logger.ConsumerLog.Warnf("GetNSSelection failed: %+v", err)
		return nil, nil, err
	}
	return res, nil, nil
}
