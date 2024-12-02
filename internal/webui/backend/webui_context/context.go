package webui_context

import (
	"github.com/omec-project/openapi/models"
)

var webuiContext = WEBUIContext{}

type WEBUIContext struct {
	NFProfiles     []models.NfProfile
	NFOamInstances []NfOamInstance
}

type NfOamInstance struct {
	NfId   string
	NfType models.NfType
	Uri    string
}

func (context *WEBUIContext) GetOamUris(targetNfType models.NfType) (uris []string) {
	for _, oamInstance := range context.NFOamInstances {
		if oamInstance.NfType == targetNfType {
			uris = append(uris, oamInstance.Uri)
			break
		}
	}
	return
}

func WEBUI_Self() *WEBUIContext {
	return &webuiContext
}
