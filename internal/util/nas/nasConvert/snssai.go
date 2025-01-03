// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasConvert

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/omec-project/openapi/models"
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
			logger.UtilLog.Warnf("decode snssai.sd failed: %+v", err)
		} else {
			buf = append(buf, byteArray...)
		}
	}
	return buf
}

func RejectedSnssaiToNas(snssai models.Snssai, rejectCause uint8) []uint8 {
	var rejectedSnssai []uint8

	if snssai.Sd == "" {
		rejectedSnssai = append(rejectedSnssai, (0x01<<4)+rejectCause)
		rejectedSnssai = append(rejectedSnssai, uint8(snssai.Sst))
	} else {
		rejectedSnssai = append(rejectedSnssai, (0x04<<4)+rejectCause)
		rejectedSnssai = append(rejectedSnssai, uint8(snssai.Sst))
		if sDBytes, err := hex.DecodeString(snssai.Sd); err != nil {
			logger.UtilLog.Warnf("decode snssai.sd failed: %+v", err)
		} else {
			rejectedSnssai = append(rejectedSnssai, sDBytes...)
		}
	}

	return rejectedSnssai
}
