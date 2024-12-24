package ausf

import (
	"github.com/ellanetworks/core/internal/ausf/context"
	"github.com/ellanetworks/core/internal/ausf/factory"
)

const AUSF_GROUP_ID = "ausfGroup001"

func Start() error {
	configuration := factory.Configuration{
		GroupId: AUSF_GROUP_ID,
	}

	factory.InitConfigFactory(configuration)
	context.Init()
	return nil
}
