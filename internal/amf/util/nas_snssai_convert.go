package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasType"
)

func SnssaiToModels(nasSnssai *nasType.SNSSAI) (snssai models.Snssai) {
	sD := nasSnssai.GetSD()
	snssai.Sd = hex.EncodeToString(sD[:])
	snssai.Sst = int32(nasSnssai.GetSST())
	return
}

func SnssaiToNas(snssai models.Snssai) []uint8 {
	var buf []uint8

	if snssai.Sd == "" {
		buf = append(buf, 0x01)
		buf = append(buf, uint8(snssai.Sst))
	} else {
		buf = append(buf, 0x04)
		buf = append(buf, uint8(snssai.Sst))
		if byteArray, err := hex.DecodeString(snssai.Sd); err != nil {
			logger.AmfLog.Warnf("decode snssai.sd failed: %+v", err)
		} else {
			buf = append(buf, byteArray...)
		}
	}
	return buf
}
