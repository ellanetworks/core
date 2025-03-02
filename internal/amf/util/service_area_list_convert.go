package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi/models"
)

// TS 24.501 9.11.3.49
func PartialServiceAreaListToNas(plmnID models.PlmnId, serviceAreaRestriction coreModels.ServiceAreaRestriction) []byte {
	var partialServiceAreaList []byte
	var allowedType uint8

	if serviceAreaRestriction.RestrictionType == coreModels.RestrictionType_ALLOWED_AREAS {
		allowedType = nasMessage.AllowedTypeAllowedArea
	} else {
		allowedType = nasMessage.AllowedTypeNonAllowedArea
	}

	numOfElements := uint8(len(serviceAreaRestriction.Areas))

	firstByte := (allowedType<<7)&0x80 + numOfElements // only support TypeOfList '00' now
	plmnIDNas := PlmnIDToNas(plmnID)

	partialServiceAreaList = append(partialServiceAreaList, firstByte)
	partialServiceAreaList = append(partialServiceAreaList, plmnIDNas...)

	for _, area := range serviceAreaRestriction.Areas {
		for _, tac := range area.Tacs {
			if tacBytes, err := hex.DecodeString(tac); err != nil {
				logger.AmfLog.Warnf("decode tac failed: %+v", err)
			} else {
				partialServiceAreaList = append(partialServiceAreaList, tacBytes...)
			}
		}
	}
	return partialServiceAreaList
}
