package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

func SpareHalfOctetAndNgksiToNas(ngKsiModels models.NgKsi) nasType.SpareHalfOctetAndNgksi {
	var ngKsiNas nasType.SpareHalfOctetAndNgksi
	switch ngKsiModels.Tsc {
	case models.ScTypeNative:
		ngKsiNas.SetTSC(nasMessage.TypeOfSecurityContextFlagNative)
	case models.ScTypeMapped:
		ngKsiNas.SetTSC(nasMessage.TypeOfSecurityContextFlagMapped)
	}

	ngKsiNas.SetSpareHalfOctet(0)
	ngKsiNas.SetNasKeySetIdentifiler(uint8(ngKsiModels.Ksi))
	return ngKsiNas
}
