// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	return amfUE.State() == Registered
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

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.state != Registered || ue.ranUe == nil || ue.ranUe.radio == nil {
		return ""
	}

	return ue.ranUe.radio.Name
}

// LastSeenAtForSubscriber returns the last-seen timestamp for a subscriber, or
// the zero time if not available.
//
// The AMF mutex is released before acquiring the UE mutex, so a UE that
// deregisters between the two locks may yield a stale timestamp. The field is
// advisory, and the next poll returns zero once the UE is fully removed.
func (amf *AMF) LastSeenAtForSubscriber(supi etsi.SUPI) time.Time {
	amf.mu.RLock()
	ue, ok := amf.UEs[supi]
	amf.mu.RUnlock()

	if !ok {
		return time.Time{}
	}

	return ue.lastSeenTime()
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
		ue.mu.Lock()
		match := ue.state == Registered && ue.ranUe != nil && ue.ranUe.radio != nil && ue.ranUe.radio.Name == radioName
		ue.mu.Unlock()

		if match && ue.supi.IsValid() && ue.supi.IsIMSI() {
			imsis = append(imsis, ue.supi.IMSI())
		}
	}

	return imsis
}
