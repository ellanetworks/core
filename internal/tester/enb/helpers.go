package enb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func GetEUTRACellIdentity(enbID string) (ngapType.EUTRACellIdentity, error) {
	enbIDBytes, err := gnb.GetGnbIdInBytes(enbID)
	if err != nil {
		return ngapType.EUTRACellIdentity{}, fmt.Errorf("could not get EUTRACellIdentity: %v", err)
	}

	slice := make([]byte, 1)

	return ngapType.EUTRACellIdentity{
		Value: aper.BitString{
			Bytes:     append(enbIDBytes, slice...),
			BitLength: 28,
		},
	}, nil
}
