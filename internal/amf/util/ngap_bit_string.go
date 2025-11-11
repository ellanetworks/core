package util

import (
	"encoding/hex"

	"github.com/free5gc/aper"
)

func BitStringToHex(bitString *aper.BitString) string {
	hexString := hex.EncodeToString(bitString.Bytes)
	hexLen := (bitString.BitLength + 3) / 4
	hexString = hexString[:hexLen]
	return hexString
}
