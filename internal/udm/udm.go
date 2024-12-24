package udm

import (
	"github.com/ellanetworks/core/internal/udm/context"
	"github.com/ellanetworks/core/internal/util/suci"
	"github.com/omec-project/openapi/models"
)

const (
	UDM_HNP_PRIVATE_KEY = "c09c17bddf23357f614f492075b970d825767718114f59554ce2f345cf8c4b6a"
)

func Start() error {
	self := context.UDM_Self()
	self.UriScheme = models.UriScheme_HTTP
	self.SuciProfiles = []suci.SuciProfile{
		{
			ProtectionScheme: "1", // Standard defined value for Protection Scheme A (TS 33.501 Annex C)
			PrivateKey:       UDM_HNP_PRIVATE_KEY,
		},
	}
	return nil
}
