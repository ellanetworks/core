package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
)

func SpareHalfOctetAndNgksiToNas(ngKsiModels models.NgKsi) (ngKsiNas nasType.SpareHalfOctetAndNgksi) {
	switch ngKsiModels.Tsc {
	case models.ScType_NATIVE:
		ngKsiNas.SetTSC(nasMessage.TypeOfSecurityContextFlagNative)
	case models.ScType_MAPPED:
		ngKsiNas.SetTSC(nasMessage.TypeOfSecurityContextFlagMapped)
	}

	ngKsiNas.SetSpareHalfOctet(0)
	ngKsiNas.SetNasKeySetIdentifiler(uint8(ngKsiModels.Ksi))
	return
}
