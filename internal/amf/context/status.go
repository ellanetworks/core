package context

import "github.com/ellanetworks/core/etsi"

func (amf *AMF) IsSubscriberRegistered(supi etsi.SUPI) bool {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	amfUE, ok := amf.UEs[supi]
	if !ok {
		return false
	}

	return amfUE.GetState() == Registered
}
