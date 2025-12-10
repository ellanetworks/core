package context

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

	if amfUe.State == nil {
		return false
	}

	currentState := amfUe.State.Current()

	return currentState == "Registered"
}
