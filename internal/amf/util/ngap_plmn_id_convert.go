package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func PlmnIDToModels(ngapPlmnID ngapType.PLMNIdentity) models.PlmnID {
	value := ngapPlmnID.Value
	hexString := strings.Split(hex.EncodeToString(value), "")
	var modelsPlmnid models.PlmnID
	modelsPlmnid.Mcc = hexString[1] + hexString[0] + hexString[3]
	if hexString[2] == "f" {
		modelsPlmnid.Mnc = hexString[5] + hexString[4]
	} else {
		modelsPlmnid.Mnc = hexString[2] + hexString[5] + hexString[4]
	}
	return modelsPlmnid
}

func PlmnIDToNgap(modelsPlmnid models.PlmnID) (*ngapType.PLMNIdentity, error) {
	var hexString string
	mcc := strings.Split(modelsPlmnid.Mcc, "")
	mnc := strings.Split(modelsPlmnid.Mnc, "")

	if len(modelsPlmnid.Mnc) == 2 {
		hexString = mcc[1] + mcc[0] + "f" + mcc[2] + mnc[1] + mnc[0]
	} else {
		hexString = mcc[1] + mcc[0] + mnc[0] + mcc[2] + mnc[2] + mnc[1]
	}

	plmnID, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("error decoding hex string: %s", err)
	}

	var ngapPlmnID ngapType.PLMNIdentity

	ngapPlmnID.Value = plmnID

	return &ngapPlmnID, nil
}
