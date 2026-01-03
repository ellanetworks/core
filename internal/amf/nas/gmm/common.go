package gmm

import (
	"github.com/ellanetworks/core/internal/models"
)

func plmnIDStringToModels(plmnIDStr string) models.PlmnID {
	return models.PlmnID{
		Mcc: plmnIDStr[:3],
		Mnc: plmnIDStr[3:],
	}
}
