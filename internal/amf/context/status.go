package context

func IsSubscriberRegistered(imsi string) bool {
	amfCtx := AMFSelf()
	amfCtx.Mutex.Lock()
	defer amfCtx.Mutex.Unlock()

	amfUE, ok := amfCtx.UEs[imsi]
	if !ok {
		return false
	}

	return amfUE.State == Registered
}
