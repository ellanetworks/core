package context

import "github.com/ellanetworks/core/internal/models"

func IsSubscriberRegistered(imsi string) bool {
	amfCtx := AMFSelf()

	ueInfo, ok := amfCtx.UePool.Load(imsi)
	if !ok {
		return false
	}

	amfUe := ueInfo.(*AmfUe)
	if amfUe == nil {
		return false
	}

	state, ok := amfUe.State[models.AccessType3GPPAccess]
	if !ok {
		return false
	}

	currentState := state.Current()

	return currentState == "Registered"
}
