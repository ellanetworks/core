package util

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/aper"
)

func BitStringToHex(bitString *aper.BitString) (hexString string) {
	hexString = hex.EncodeToString(bitString.Bytes)
	hexLen := (bitString.BitLength + 3) / 4
	hexString = hexString[:hexLen]
	return
}

func HexToBitString(hexString string, bitLength int) (bitString aper.BitString) {
	hexLen := len(hexString)
	if hexLen != (bitLength+3)/4 {
		logger.AmfLog.Warnf("hexLen[%d] doesn't match bitLength[%d]", hexLen, bitLength)
		return
	}
	if hexLen%2 == 1 {
		hexString += "0"
	}
	if byteTmp, err := hex.DecodeString(hexString); err != nil {
		logger.AmfLog.Warnf("Decode byteString failed: %+v", err)
	} else {
		bitString.Bytes = byteTmp
	}
	bitString.BitLength = uint64(bitLength)
	mask := byte(0xff)
	mask = mask << uint(8-bitLength%8)
	if mask != 0 {
		bitString.Bytes[len(bitString.Bytes)-1] &= mask
	}
	return
}

func ByteToBitString(byteArray []byte, bitLength int) (bitString aper.BitString) {
	byteLen := (bitLength + 7) / 8
	if byteLen > len(byteArray) {
		logger.AmfLog.Warnf("bitLength[%d] is beyond byteArray size[%d]", bitLength, len(byteArray))
		return
	}
	bitString.Bytes = byteArray
	bitString.BitLength = uint64(bitLength)
	return
}
