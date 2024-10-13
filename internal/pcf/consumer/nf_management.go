package consumer

import (
	"github.com/omec-project/openapi/models"
	pcf_context "github.com/yeastengine/ella/internal/pcf/context"
)

func BuildNFInstance(context *pcf_context.PCFContext) (profile models.NfProfile, err error) {
	profile.NfInstanceId = context.NfId
	profile.NfType = models.NfType_PCF
	profile.NfStatus = models.NfStatus_REGISTERED
	profile.Ipv4Addresses = append(profile.Ipv4Addresses, context.BindingIPv4)
	service := []models.NfService{}
	for _, nfService := range context.NfService {
		service = append(service, nfService)
	}
	profile.NfServices = &service

	var plmns []models.PlmnId
	for _, plmnItem := range context.PlmnList {
		plmns = append(plmns, plmnItem.PlmnId)
	}
	if len(plmns) > 0 {
		profile.PlmnList = &plmns
	}

	profile.PcfInfo = &models.PcfInfo{
		DnnList: context.DnnList,
	}
	return profile, err
}
