package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
)

func TaiToModels(tai ngapType.TAI) models.Tai {
	var modelsTai models.Tai
	plmnID := PlmnIdToModels(tai.PLMNIdentity)
	modelsTai.PlmnId = &plmnID
	modelsTai.Tac = hex.EncodeToString(tai.TAC.Value)
	return modelsTai
}
