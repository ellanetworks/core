package util

import (
	"encoding/hex"
	"reflect"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

// TS 24.501 9.11.3.9
func TaiListToNas(taiList []models.Tai) []uint8 {
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
		plmnNas := PlmnIDToNas(*plmnId)
		taiListNas = append(taiListNas, plmnNas...)

		for _, tai := range taiList {
			if tacBytes, err := hex.DecodeString(tai.Tac); err != nil {
				logger.AmfLog.Warnf("decode tac failed: %+v", err)
			} else {
				taiListNas = append(taiListNas, tacBytes...)
			}
		}
	case 0x02:
		for _, tai := range taiList {
			plmnNas := PlmnIDToNas(*tai.PlmnId)
			if tacBytes, err := hex.DecodeString(tai.Tac); err != nil {
				logger.AmfLog.Warnf("decode tac failed: %+v", err)
			} else {
				taiListNas = append(taiListNas, plmnNas...)
				taiListNas = append(taiListNas, tacBytes...)
			}
		}
	}

	return taiListNas
}
