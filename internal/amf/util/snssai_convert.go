package util

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
)

func SNssaiToModels(ngapSnssai ngapType.SNSSAI) models.Snssai {
	var modelsSnssai models.Snssai
	modelsSnssai.Sst = int32(ngapSnssai.SST.Value[0])
	if ngapSnssai.SD != nil {
		modelsSnssai.Sd = hex.EncodeToString(ngapSnssai.SD.Value)
	}
	return modelsSnssai
}

func SNssaiToNgap(modelsSnssai models.Snssai) (ngapType.SNSSAI, error) {
	var ngapSnssai ngapType.SNSSAI

	ngapSnssai.SST.Value = []byte{byte(modelsSnssai.Sst)}

	if modelsSnssai.Sd != "" {
		ngapSnssai.SD = new(ngapType.SD)
		sdTmp, err := hex.DecodeString(modelsSnssai.Sd)
		if err != nil {
			return ngapSnssai, fmt.Errorf("could not decode SD: %+v", err)
		}
		ngapSnssai.SD.Value = sdTmp
	}

	return ngapSnssai, nil
}
