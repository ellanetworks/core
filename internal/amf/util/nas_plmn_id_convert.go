package util

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/models"
)

func PlmnIDToNas(plmnID models.PlmnID) ([]uint8, error) {
	var plmnNas []uint8

	var mccDigit1, mccDigit2, mccDigit3 int

	mccDigitTmp, err := strconv.Atoi(string(plmnID.Mcc[0]))
	if err != nil {
		return nil, fmt.Errorf("atoi mcc error: %+v", err)
	}

	mccDigit1 = mccDigitTmp

	mccDigitTmp, err = strconv.Atoi(string(plmnID.Mcc[1]))
	if err != nil {
		return nil, fmt.Errorf("atoi mcc error: %+v", err)
	}

	mccDigit2 = mccDigitTmp

	mccDigitTmp, err = strconv.Atoi(string(plmnID.Mcc[2]))
	if err != nil {
		return nil, fmt.Errorf("atoi mcc error: %+v", err)
	}

	mccDigit3 = mccDigitTmp

	var mncDigit1, mncDigit2, mncDigit3 int

	mncDigitTmp, err := strconv.Atoi(string(plmnID.Mnc[0]))
	if err != nil {
		return nil, fmt.Errorf("atoi mnc error: %+v", err)
	}

	mncDigit1 = mncDigitTmp

	mncDigitTmp, err = strconv.Atoi(string(plmnID.Mnc[1]))
	if err != nil {
		return nil, fmt.Errorf("atoi mnc error: %+v", err)
	}

	mncDigit2 = mncDigitTmp
	mncDigit3 = 0x0f

	if len(plmnID.Mnc) == 3 {
		mncDigitTmp, err := strconv.Atoi(string(plmnID.Mnc[2]))
		if err != nil {
			return nil, fmt.Errorf("atoi mnc error: %+v", err)
		}

		mncDigit3 = mncDigitTmp
	}

	plmnNas = []uint8{
		uint8((mccDigit2 << 4) | mccDigit1),
		uint8((mncDigit3 << 4) | mccDigit3),
		uint8((mncDigit2 << 4) | mncDigit1),
	}

	return plmnNas, nil
}
