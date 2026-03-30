package gmm

import (
	"github.com/ellanetworks/core/internal/models"
)

// nextNgKsi returns the next available NAS Key Set Identifier.
// KSI is a 3-bit field (0–6 valid, 7 means "no key available").
// See 3GPP TS 24.501 section 9.11.3.32.
func nextNgKsi(current int32) int32 {
	if current >= 0 && current < 6 {
		return current + 1
	}

	return 0
}

func plmnIDStringToModels(plmnIDStr string) models.PlmnID {
	if len(plmnIDStr) < 5 {
		return models.PlmnID{}
	}

	return models.PlmnID{
		Mcc: plmnIDStr[:3],
		Mnc: plmnIDStr[3:],
	}
}
