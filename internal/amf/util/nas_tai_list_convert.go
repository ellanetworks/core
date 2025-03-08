package util

import (
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/models"
)

// TS 24.501 9.11.3.9
func TaiListToNas(taiList []models.Tai) ([]uint8, error) {
	var taiListNas []uint8
	typeOfList := 0x00

	plmnId := taiList[0].PlmnId
	for _, tai := range taiList {
		if !reflect.DeepEqual(plmnId, tai.PlmnId) {
			typeOfList = 0x02
		}
	}

	numOfElementsNas := uint8(len(taiList)) - 1

	taiListNas = append(taiListNas, uint8(typeOfList<<5)+numOfElementsNas)

	switch typeOfList {
	case 0x00:
		plmnNas, err := PlmnIDToNas(*plmnId)
		if err != nil {
			return nil, fmt.Errorf("failed to convert plmnID to nas: %s", err)
		}
		taiListNas = append(taiListNas, plmnNas...)

		for _, tai := range taiList {
			tacBytes, err := hex.DecodeString(tai.Tac)
			if err != nil {
				return nil, fmt.Errorf("failed to decode tac: %s", err)
			}
			taiListNas = append(taiListNas, tacBytes...)
		}
	case 0x02:
		for _, tai := range taiList {
			plmnNas, err := PlmnIDToNas(*tai.PlmnId)
			if err != nil {
				return nil, fmt.Errorf("failed to convert plmnID to nas: %s", err)
			}
			tacBytes, err := hex.DecodeString(tai.Tac)
			if err != nil {
				return nil, fmt.Errorf("failed to decode tac: %s", err)
			}
			taiListNas = append(taiListNas, plmnNas...)
			taiListNas = append(taiListNas, tacBytes...)
		}
	}

	return taiListNas, nil
}
