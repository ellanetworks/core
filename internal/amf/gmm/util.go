// Copyright 2024 Ella Networks
package gmm

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
)

func AnTypeToNas(anType models.AccessType) uint8 {
	if anType == models.AccessType__3_GPP_ACCESS {
		return nasMessage.AccessType3GPP
	} else if anType == models.AccessType_NON_3_GPP_ACCESS {
		return nasMessage.AccessTypeNon3GPP
	}

	return nasMessage.AccessTypeBoth
}
