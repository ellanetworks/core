package util

import (
	"encoding/hex"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
)

func PlmnIDToModels(ngapPlmnID ngapType.PLMNIdentity) (modelsPlmnid models.PlmnID) {
	value := ngapPlmnID.Value
	hexString := strings.Split(hex.EncodeToString(value), "")
	modelsPlmnid.Mcc = hexString[1] + hexString[0] + hexString[3]
	if hexString[2] == "f" {
		modelsPlmnid.Mnc = hexString[5] + hexString[4]
	} else {
		modelsPlmnid.Mnc = hexString[2] + hexString[5] + hexString[4]
	}
	return
}

func PlmnIDToNgap(modelsPlmnid models.PlmnID) ngapType.PLMNIdentity {
	var hexString string
	mcc := strings.Split(modelsPlmnid.Mcc, "")
	mnc := strings.Split(modelsPlmnid.Mnc, "")
	if len(modelsPlmnid.Mnc) == 2 {
		hexString = mcc[1] + mcc[0] + "f" + mcc[2] + mnc[1] + mnc[0]
	} else {
		hexString = mcc[1] + mcc[0] + mnc[0] + mcc[2] + mnc[2] + mnc[1]
	}

	var ngapPlmnID ngapType.PLMNIdentity
	if plmnID, err := hex.DecodeString(hexString); err != nil {
		logger.AmfLog.Warnf("decode plmn failed: %+v", err)
	} else {
		ngapPlmnID.Value = plmnID
	}
	return ngapPlmnID
}
