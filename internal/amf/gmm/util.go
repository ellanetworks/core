// Copyright 2024 Ella Networks
package gmm

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

func AnTypeToNas(anType models.AccessType) uint8 {
	switch anType {
	case models.AccessType3GPPAccess:
		return nasMessage.AccessType3GPP
	case models.AccessTypeNon3GPPAccess:
		return nasMessage.AccessTypeNon3GPP
	}

	return nasMessage.AccessTypeBoth
}
