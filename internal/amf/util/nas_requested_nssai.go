package util

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
)

// TS 24.501 9.11.3.37
func RequestedNssaiToModels(nasNssai *nasType.RequestedNSSAI) ([]*models.Snssai, error) {
	var requestNssai []*models.Snssai

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
func snssaiToModels(lengthOfSnssaiContents uint8, buf []byte) (*models.Snssai, error) {
	switch lengthOfSnssaiContents {
	case 0x01: // SST
		return &models.Snssai{
			Sst: int32(buf[1]),
		}, nil
	case 0x02: // SST and mapped HPLMN SST
		return &models.Snssai{
			Sst: int32(buf[1]),
		}, nil
	case 0x04: // SST and SD
		return &models.Snssai{
			Sst: int32(buf[1]),
			Sd:  hex.EncodeToString(buf[2:5]),
		}, nil
	case 0x05: // SST, SD and mapped HPLMN SST
		return &models.Snssai{
			Sst: int32(buf[1]),
			Sd:  hex.EncodeToString(buf[2:5]),
		}, nil
	case 0x08: // SST, SD, mapped HPLMN SST and mapped HPLMN SD
		return &models.Snssai{
			Sst: int32(buf[1]),
			Sd:  hex.EncodeToString(buf[2:5]),
		}, nil
	default:
		return nil, fmt.Errorf("invalid length of S-NSSAI contents: %d", lengthOfSnssaiContents)
	}
}
