package util

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasType"
)

// TS 24.501 9.11.3.37
func RequestedNssaiToModels(nasNssai *nasType.RequestedNSSAI) ([]models.MappingOfSnssai, error) {
	var requestNssai []models.MappingOfSnssai

	buf := nasNssai.GetSNSSAIValue()
	lengthOfBuf := int(nasNssai.GetLen())
	offset := 0
	for offset < lengthOfBuf {
		lengthOfSnssaiContents := buf[offset]
		if snssai, err := snssaiToModels(lengthOfSnssaiContents, buf[offset:]); err != nil {
			return nil, err
		} else {
			requestNssai = append(requestNssai, snssai)
			// lengthOfSnssaiContents is 1 byte
			offset += int(lengthOfSnssaiContents + 1)
		}
	}

	return requestNssai, nil
}

// TS 24.501 9.11.2.8, Length & value part of S-NSSAI IE
func snssaiToModels(lengthOfSnssaiContents uint8, buf []byte) (models.MappingOfSnssai, error) {
	snssai := models.MappingOfSnssai{}

	switch lengthOfSnssaiContents {
	case 0x01: // SST
		snssai.ServingSnssai = &models.Snssai{
			Sst: int32(buf[1]),
		}
		return snssai, nil
	case 0x02: // SST and mapped HPLMN SST
		snssai.ServingSnssai = &models.Snssai{
			Sst: int32(buf[1]),
		}
		snssai.HomeSnssai = &models.Snssai{
			Sst: int32(buf[2]),
		}
		return snssai, nil
	case 0x04: // SST and SD
		snssai.ServingSnssai = &models.Snssai{
			Sst: int32(buf[1]),
			Sd:  hex.EncodeToString(buf[2:5]),
		}
		return snssai, nil
	case 0x05: // SST, SD and mapped HPLMN SST
		snssai.ServingSnssai = &models.Snssai{
			Sst: int32(buf[1]),
			Sd:  hex.EncodeToString(buf[2:5]),
		}
		snssai.HomeSnssai = &models.Snssai{
			Sst: int32(buf[5]),
		}
		return snssai, nil
	case 0x08: // SST, SD, mapped HPLMN SST and mapped HPLMN SD
		snssai.ServingSnssai = &models.Snssai{
			Sst: int32(buf[1]),
			Sd:  hex.EncodeToString(buf[2:5]),
		}
		snssai.HomeSnssai = &models.Snssai{
			Sst: int32(buf[5]),
			Sd:  hex.EncodeToString(buf[6:9]),
		}
		return snssai, nil
	default:
		return snssai, fmt.Errorf("invalid length of S-NSSAI contents: %d", lengthOfSnssaiContents)
	}
}
