package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
)

func PlmnIdToModels(ngapPlmnId ngapType.PLMNIdentity) models.PlmnId {
	value := ngapPlmnId.Value
	hexString := strings.Split(hex.EncodeToString(value), "")
	var modelsPlmnid models.PlmnId
	modelsPlmnid.Mcc = hexString[1] + hexString[0] + hexString[3]
	if hexString[2] == "f" {
		modelsPlmnid.Mnc = hexString[5] + hexString[4]
	} else {
		modelsPlmnid.Mnc = hexString[2] + hexString[5] + hexString[4]
	}
	return modelsPlmnid
}

func PlmnIdToNgap(modelsPlmnid models.PlmnId) (*ngapType.PLMNIdentity, error) {
	var hexString string
	mcc := strings.Split(modelsPlmnid.Mcc, "")
	mnc := strings.Split(modelsPlmnid.Mnc, "")
	if len(modelsPlmnid.Mnc) == 2 {
		hexString = mcc[1] + mcc[0] + "f" + mcc[2] + mnc[1] + mnc[0]
	} else {
		hexString = mcc[1] + mcc[0] + mnc[0] + mcc[2] + mnc[2] + mnc[1]
	}

	var ngapPlmnId ngapType.PLMNIdentity
	plmnId, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("error decoding hex string: %s", err)
	}
	ngapPlmnId.Value = plmnId
	return &ngapPlmnId, nil
}
