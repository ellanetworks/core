// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"time"
)

// ConnectedSubscriber is a consistent snapshot of a 5G subscriber's live
// status for the status API, taken under a single registry lock so the fields cannot
// tear across per-field reads.
type ConnectedSubscriber struct {
	RadioName   string
	NumSessions int
	LastSeenAt  time.Time
	Connected   bool
	Registered  bool
}

// ConnectedSubscribers returns a snapshot of every 5G subscriber with a 5GMM context keyed by
// IMSI, built under one amf.mu.RLock so it is a single consistent read that cannot tear
// against a concurrent register/idle transition.
func (amf *AMF) ConnectedSubscribers() map[string]ConnectedSubscriber {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	out := make(map[string]ConnectedSubscriber)

	for _, ue := range amf.UEs {
		if !ue.supi.IsValid() || !ue.supi.IsIMSI() {
			continue
		}

		ue.mu.Lock()
		state := ue.state

		radioName := ""
		conn := ue.active.Load()

		if conn != nil {
			radioName = conn.radioName()
		}

		numSessions := len(ue.SmContextList)
		ue.mu.Unlock()

		if state == Deregistered {
			continue
		}

		out[ue.supi.IMSI()] = ConnectedSubscriber{
			RadioName:   radioName,
			NumSessions: numSessions,
			LastSeenAt:  ue.lastSeenTime(),
			Connected:   conn != nil,
			Registered:  state == Registered,
		}
	}

	return out
}
