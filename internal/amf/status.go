package amf

import (
	"time"

	"github.com/ellanetworks/core/etsi"
)

func (amf *AMF) IsSubscriberRegistered(supi etsi.SUPI) bool {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	amfUE, ok := amf.UEs[supi]
	if !ok {
		return false
	}

	return amfUE.GetState() == Registered
}

// RadioNameForSubscriber returns the radio name for a registered subscriber,
// or an empty string if the subscriber is not registered or has no radio.
func (amf *AMF) RadioNameForSubscriber(supi etsi.SUPI) string {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	ue, ok := amf.UEs[supi]
	if !ok || ue.GetState() != Registered || ue.RanUe == nil || ue.RanUe.Radio == nil {
		return ""
	}

	return ue.RanUe.Radio.Name
}

// LastSeenAtForSubscriber returns the last-seen timestamp for a registered
// subscriber, or the zero time if not available.
//
// Note: the AMF mutex is released before acquiring the UE mutex.  Between the
// two locks the UE could deregister, so we may return a stale LastSeenAt for a
// UE that has just left.  This is acceptable — the field is advisory and the
// caller will get a zero time on the next poll once the UE is fully removed.
func (amf *AMF) LastSeenAtForSubscriber(supi etsi.SUPI) time.Time {
	amf.Mutex.Lock()
	ue, ok := amf.UEs[supi]
	amf.Mutex.Unlock()

	if !ok {
		return time.Time{}
	}

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	return ue.LastSeenAt
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
