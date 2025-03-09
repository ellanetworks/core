// Copyright 2024 Ella Networks
package gmm

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
)

func AnTypeToNas(anType models.AccessType) uint8 {
	if anType == models.AccessType3GPPAccess {
		return nasMessage.AccessType3GPP
	} else if anType == models.AccessTypeNon3GPPAccess {
		return nasMessage.AccessTypeNon3GPP
	}

	return nasMessage.AccessTypeBoth
}
