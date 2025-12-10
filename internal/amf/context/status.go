package context

func IsSubscriberRegistered(imsi string) bool {
	amfCtx := AMFSelf()
	amfCtx.Mutex.Lock()
	defer amfCtx.Mutex.Unlock()

	amfUe, ok := amfCtx.UePool[imsi]
	if !ok {
		return false
	}

	if amfUe.State == nil {
		return false
	}

	currentState := amfUe.State.Current()

	return currentState == "Registered"
}
