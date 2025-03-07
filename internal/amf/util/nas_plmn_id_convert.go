package util

import (
	"encoding/hex"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func PlmnIDToNas(plmnID models.PlmnID) []uint8 {
	var plmnNas []uint8

	var mccDigit1, mccDigit2, mccDigit3 int
	if mccDigitTmp, err := strconv.Atoi(string(plmnID.Mcc[0])); err != nil {
		logger.AmfLog.Warnf("atoi mcc error: %+v", err)
	} else {
		mccDigit1 = mccDigitTmp
	}
	if mccDigitTmp, err := strconv.Atoi(string(plmnID.Mcc[1])); err != nil {
		logger.AmfLog.Warnf("atoi mcc error: %+v", err)
	} else {
		mccDigit2 = mccDigitTmp
	}
	if mccDigitTmp, err := strconv.Atoi(string(plmnID.Mcc[2])); err != nil {
		logger.AmfLog.Warnf("atoi mcc error: %+v", err)
	} else {
		mccDigit3 = mccDigitTmp
	}

	var mncDigit1, mncDigit2, mncDigit3 int
	if mncDigitTmp, err := strconv.Atoi(string(plmnID.Mnc[0])); err != nil {
		logger.AmfLog.Warnf("atoi mnc error: %+v", err)
	} else {
		mncDigit1 = mncDigitTmp
	}
	if mncDigitTmp, err := strconv.Atoi(string(plmnID.Mnc[1])); err != nil {
		logger.AmfLog.Warnf("atoi mnc error: %+v", err)
	} else {
		mncDigit2 = mncDigitTmp
	}
	mncDigit3 = 0x0f
	if len(plmnID.Mnc) == 3 {
		if mncDigitTmp, err := strconv.Atoi(string(plmnID.Mnc[2])); err != nil {
			logger.AmfLog.Warnf("atoi mn error: %+v", err)
		} else {
			mncDigit3 = mncDigitTmp
		}
	}

	plmnNas = []uint8{
		uint8((mccDigit2 << 4) | mccDigit1),
		uint8((mncDigit3 << 4) | mccDigit3),
		uint8((mncDigit2 << 4) | mncDigit1),
	}

	return plmnNas
}

func PlmnIDToString(nasBuf []byte) string {
	mccDigit1 := nasBuf[0] & 0x0f
	mccDigit2 := (nasBuf[0] & 0xf0) >> 4
	mccDigit3 := (nasBuf[1] & 0x0f)

	mncDigit1 := (nasBuf[2] & 0x0f)
	mncDigit2 := (nasBuf[2] & 0xf0) >> 4
	mncDigit3 := (nasBuf[1] & 0xf0) >> 4

	tmpBytes := []byte{(mccDigit1 << 4) | mccDigit2, (mccDigit3 << 4) | mncDigit1, (mncDigit2 << 4) | mncDigit3}

	plmnID := hex.EncodeToString(tmpBytes)
	if plmnID[5] == 'f' {
		plmnID = plmnID[:5] // get plmnID[0~4]
	}
	return plmnID
}
