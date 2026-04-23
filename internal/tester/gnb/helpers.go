package gnb

import (
	"encoding/hex"
	"fmt"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"github.com/free5gc/openapi/models"
)

func GetTacInBytes(tacStr string) ([]byte, error) {
	resu, err := hex.DecodeString(tacStr)
	if err != nil {
		return nil, fmt.Errorf("could not decode tac to bytes: %v", err)
	}

	return resu, nil
}

func GetSliceInBytes(sst int32, sd string) ([]byte, []byte, error) {
	sstBytes := []byte{byte(sst)}

	if sd != "" {
		sdBytes, err := hex.DecodeString(sd)
		if err != nil {
			return sstBytes, nil, fmt.Errorf("could not decode sd to bytes: %v", err)
		}

		return sstBytes, sdBytes, nil
	}

	return sstBytes, nil, nil
}

func GetPLMNIdentity(mcc string, mnc string) ngapType.PLMNIdentity {
	return ngapConvert.PlmnIdToNgap(models.PlmnId{Mcc: mcc, Mnc: mnc})
}

func GetMccAndMncInOctets(mccStr string, mncStr string) ([]byte, error) {
	var res string

	// reverse mcc and mnc
	mcc := reverse(mccStr)
	mnc := reverse(mncStr)

	if len(mnc) == 2 {
		res = fmt.Sprintf("%c%cf%c%c%c", mcc[1], mcc[2], mcc[0], mnc[0], mnc[1])
	} else {
		res = fmt.Sprintf("%c%c%c%c%c%c", mcc[1], mcc[2], mnc[2], mcc[0], mnc[0], mnc[1])
	}

	resu, err := hex.DecodeString(res)
	if err != nil {
		return nil, fmt.Errorf("could not decode mcc/mnc to octets: %v", err)
	}

	return resu, nil
}

func reverse(s string) string {
	var aux string
	for _, valor := range s {
		aux = string(valor) + aux
	}

	return aux
}

func GetNRCellIdentity(gnbID string) (ngapType.NRCellIdentity, error) {
	nci, err := GetGnbIdInBytes(gnbID)
	if err != nil {
		return ngapType.NRCellIdentity{}, fmt.Errorf("could not get NRCellIdentity: %v", err)
	}

	slice := make([]byte, 2)

	return ngapType.NRCellIdentity{
		Value: aper.BitString{
			Bytes:     append(nci, slice...),
			BitLength: 36,
		},
	}, nil
}

func GetGnbIdInBytes(gnbId string) ([]byte, error) {
	resu, err := hex.DecodeString(gnbId)
	if err != nil {
		return nil, fmt.Errorf("could not decode gnbId to bytes: %v", err)
	}

	return resu, nil
}
