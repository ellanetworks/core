// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasConvert

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

// subclause 9.11.3.53A in 3GPP TS 24.501.
func UpuInfoToNas(upuInfo models.UpuInfo) []uint8 {
	var buf []uint8

	// set upu Header
	buf = append(buf, upuInfoGetHeader(upuInfo.UpuRegInd, upuInfo.UpuAckInd))
	// Set UPU-MAC-IAUSF
	if byteArray, err := hex.DecodeString(upuInfo.UpuMacIausf); err != nil {
		logger.UtilLog.Warnf("decode upuInfo.UpuMacIausf failed: %+v", err)
	} else {
		buf = append(buf, byteArray...)
		// Set Counter UPU
		if computerUpuByteArray, errUpu := hex.DecodeString(upuInfo.CounterUpu); errUpu != nil {
			logger.UtilLog.Warnf("decode upuInfo.CounterUpu failed: %+v", errUpu)
		} else {
			buf = append(buf, computerUpuByteArray...)
		}
	}
	// Set UE parameters update list
	for _, data := range upuInfo.UpuDataList {
		var byteArray []byte
		if data.SecPacket != "" {
			buf = append(buf, 0x01)
			if byteArrayTmp, err := hex.DecodeString(data.SecPacket); err != nil {
				logger.UtilLog.Warnf("decode data.SecPacket failed: %+v", err)
			} else {
				byteArray = byteArrayTmp
			}
		} else {
			buf = append(buf, 0x02)
			byteArray = []byte{}
			for _, snssai := range data.DefaultConfNssai {
				snssaiData := SnssaiToNas(snssai)
				byteArray = append(byteArray, snssaiData...)
			}
		}
		buf = append(buf, uint8(len(byteArray)))
		buf = append(buf, byteArray...)
	}
	return buf
}

func upuInfoGetHeader(reg bool, ack bool) (buf uint8) {
	var regValue, ackValue uint8
	if reg {
		regValue = 1
	}
	if ack {
		ackValue = 1
	}
	buf = regValue<<2 + ackValue<<1
	return
}

func UpuAckToModels(buf []uint8) (string, error) {
	if (buf[0] != 0x01) || (len(buf) != 17) {
		return "", fmt.Errorf("NAS UPU Ack is not valid")
	}
	return hex.EncodeToString(buf[1:]), nil
}
