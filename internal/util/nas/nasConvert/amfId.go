// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasConvert

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
)

func AmfIdToNas(amfId string) (amfRegionId uint8, amfSetId uint16, amfPointer uint8) {
	amfIdBytes, err := hex.DecodeString(amfId)
	if err != nil {
		logger.UtilLog.Errorf("amfId decode failed: %+v", err)
	}

	amfRegionId = amfIdBytes[0]
	amfSetId = uint16(amfIdBytes[1])<<2 + (uint16(amfIdBytes[2])&0x00c0)>>6
	amfPointer = amfIdBytes[2] & 0x3f
	return
}
