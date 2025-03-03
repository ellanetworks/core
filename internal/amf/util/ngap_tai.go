package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
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

func TaiToNgap(tai models.Tai) ngapType.TAI {
	var ngapTai ngapType.TAI

	ngapTai.PLMNIdentity = PlmnIdToNgap(*tai.PlmnId)
	if tac, err := hex.DecodeString(tai.Tac); err != nil {
		logger.AmfLog.Warnf("decode TAC failed: %+v", err)
	} else {
		ngapTai.TAC.Value = tac
	}
	return ngapTai
}
