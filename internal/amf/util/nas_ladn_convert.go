package util

import (
	"github.com/ellanetworks/core/internal/models"
)

func LadnToNas(dnn string, taiLists []models.Tai) (ladnNas []uint8) {
	dnnNas := []byte(dnn)

	ladnNas = append(ladnNas, uint8(len(dnnNas)))
	ladnNas = append(ladnNas, dnnNas...)

	taiListNas := TaiListToNas(taiLists)
	ladnNas = append(ladnNas, uint8(len(taiListNas)))
	ladnNas = append(ladnNas, taiListNas...)
	return
}
