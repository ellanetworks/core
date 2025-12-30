package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
)

func SnssaiToModels(n *nasType.SNSSAI) *models.Snssai {
	var out models.Snssai
	out.Sst = int32(n.GetSST())

	if n.Len >= 4 {
		sd := n.Octet[1:4] // 3 bytes following SST
		out.Sd = strings.ToUpper(hex.EncodeToString(sd))
	} else {
		out.Sd = ""
	}

	return &out
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
