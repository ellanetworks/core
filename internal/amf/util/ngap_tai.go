package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func TaiToModels(tai ngapType.TAI) models.Tai {
	var modelsTai models.Tai
	plmnID := PlmnIDToModels(tai.PLMNIdentity)
	modelsTai.PlmnID = &plmnID
	modelsTai.Tac = hex.EncodeToString(tai.TAC.Value)
	return modelsTai
}
