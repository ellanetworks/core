package server

import (
	"github.com/omec-project/openapi/models"
)

var nmsContext = NMSContext{}

type NMSContext struct {
	NFProfiles     []models.NfProfile
	NFOamInstances []NfOamInstance
}

type NfOamInstance struct {
	NfId   string
	NfType models.NfType
	Uri    string
}

func (context *NMSContext) GetOamUris(targetNfType models.NfType) (uris []string) {
	for _, oamInstance := range context.NFOamInstances {
		if oamInstance.NfType == targetNfType {
			uris = append(uris, oamInstance.Uri)
			break
		}
	}
	return
}

func NMS_Self() *NMSContext {
	return &nmsContext
}
