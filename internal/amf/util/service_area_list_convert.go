package util

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
)

// TS 24.501 9.11.3.49
func PartialServiceAreaListToNas(plmnID models.PlmnId, serviceAreaRestriction models.ServiceAreaRestriction) ([]byte, error) {
	var partialServiceAreaList []byte
	var allowedType uint8

	if serviceAreaRestriction.RestrictionType == models.RestrictionType_ALLOWED_AREAS {
		allowedType = nasMessage.AllowedTypeAllowedArea
	} else {
		allowedType = nasMessage.AllowedTypeNonAllowedArea
	}

	numOfElements := uint8(len(serviceAreaRestriction.Areas))

	firstByte := (allowedType<<7)&0x80 + numOfElements // only support TypeOfList '00' now
	plmnIDNas, err := PlmnIDToNas(plmnID)
	if err != nil {
		return nil, fmt.Errorf("error converting plmnID to nas: %+v", err)
	}

	partialServiceAreaList = append(partialServiceAreaList, firstByte)
	partialServiceAreaList = append(partialServiceAreaList, plmnIDNas...)

	for _, area := range serviceAreaRestriction.Areas {
		for _, tac := range area.Tacs {
			tacBytes, err := hex.DecodeString(tac)
			if err != nil {
				return nil, fmt.Errorf("decode tac failed: %+v", err)
			}
			partialServiceAreaList = append(partialServiceAreaList, tacBytes...)
		}
	}
	return partialServiceAreaList, nil
}
