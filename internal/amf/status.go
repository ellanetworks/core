package amf

import (
	"time"

	"github.com/ellanetworks/core/etsi"
)

func (amf *AMF) IsSubscriberRegistered(supi etsi.SUPI) bool {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	amfUE, ok := amf.UEs[supi]
	if !ok {
		return false
	}

	return amfUE.GetState() == Registered
}

// RadioNameForSubscriber returns the radio name for a registered subscriber,
// or an empty string if the subscriber is not registered or has no radio.
func (amf *AMF) RadioNameForSubscriber(supi etsi.SUPI) string {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	ue, ok := amf.UEs[supi]
	if !ok {
		return ""
	}

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if ue.state != Registered || ue.ranUe == nil || ue.ranUe.Radio == nil {
		return ""
	}

	return ue.ranUe.Radio.Name
}

// LastSeenAtForSubscriber returns the last-seen timestamp for a registered
// subscriber, or the zero time if not available.
//
// Note: the AMF mutex is released before acquiring the UE mutex.  Between the
// two locks the UE could deregister, so we may return a stale LastSeenAt for a
// UE that has just left.  This is acceptable — the field is advisory and the
// caller will get a zero time on the next poll once the UE is fully removed.
func (amf *AMF) LastSeenAtForSubscriber(supi etsi.SUPI) time.Time {
	amf.mu.RLock()
	ue, ok := amf.UEs[supi]
	amf.mu.RUnlock()

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
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	var imsis []string

	for _, ue := range amf.UEs {
		ue.Mutex.Lock()
		match := ue.state == Registered && ue.ranUe != nil && ue.ranUe.Radio != nil && ue.ranUe.Radio.Name == radioName
		ue.Mutex.Unlock()

		if match && ue.Supi.IsValid() && ue.Supi.IsIMSI() {
			imsis = append(imsis, ue.Supi.IMSI())
		}
	}

	return imsis
}
