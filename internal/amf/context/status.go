package context

func (amf *AMF) IsSubscriberRegistered(imsi string) bool {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	amfUE, ok := amf.UEs[imsi]
	if !ok {
		return false
	}

	return amfUE.State == Registered
}
