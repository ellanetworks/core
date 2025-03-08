package util

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

func LadnToNas(dnn string, taiLists []models.Tai) ([]uint8, error) {
	dnnNas := []byte(dnn)
	var ladnNas []uint8
	ladnNas = append(ladnNas, uint8(len(dnnNas)))
	ladnNas = append(ladnNas, dnnNas...)
	taiListNas, err := TaiListToNas(taiLists)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tai list to nas: %s", err)
	}
	ladnNas = append(ladnNas, uint8(len(taiListNas)))
	ladnNas = append(ladnNas, taiListNas...)
	return ladnNas, nil
}
