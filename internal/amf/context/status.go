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

// RegisteredSubscribersForRadio returns the IMSIs of all subscribers that are
// in the Registered state and whose current RAN UE association points to the
// named radio.  This is the authoritative way to count subscribers on a radio
// because it uses the same registration check as IsSubscriberRegistered.
func (amf *AMF) RegisteredSubscribersForRadio(radioName string) []string {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	var imsis []string

	for _, ue := range amf.UEs {
		if ue.GetState() != Registered {
			continue
		}

		if ue.RanUe == nil || ue.RanUe.Radio == nil {
			continue
		}

		if ue.RanUe.Radio.Name != radioName {
			continue
		}

		if ue.Supi.IsValid() && ue.Supi.IsIMSI() {
			imsis = append(imsis, ue.Supi.IMSI())
		}
	}

	return imsis
}
