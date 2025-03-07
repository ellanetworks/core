package util

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasType"
)

func SnssaiToModels(nasSnssai *nasType.SNSSAI) models.Snssai {
	var snssai models.Snssai
	sD := nasSnssai.GetSD()
	snssai.Sd = hex.EncodeToString(sD[:])
	snssai.Sst = int32(nasSnssai.GetSST())
	return snssai
}

func SnssaiToNas(snssai models.Snssai) ([]uint8, error) {
	var buf []uint8

	if snssai.Sd == "" {
		buf = append(buf, 0x01)
		buf = append(buf, uint8(snssai.Sst))
	} else {
		buf = append(buf, 0x04)
		buf = append(buf, uint8(snssai.Sst))
		byteArray, err := hex.DecodeString(snssai.Sd)
		if err != nil {
			return nil, fmt.Errorf("error decoding snssai sd: %+v", err)
		}
		buf = append(buf, byteArray...)
	}
	return buf, nil
}
