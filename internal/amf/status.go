// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"time"
)

// ConnectedSubscriber is a consistent snapshot of a Registered 5G subscriber's live
// status for the status API, taken under a single registry lock so the fields cannot
// tear across per-field reads. Mirrors the MME's ConnectedSubscriber.
type ConnectedSubscriber struct {
	RadioName   string
	NumSessions int
	LastSeenAt  time.Time
}

// ConnectedSubscribers returns a snapshot of every Registered 5G subscriber keyed by
// IMSI, built under one amf.mu.RLock — one consistent read instead of the per-field
// accessors that could tear against a concurrent register/idle transition. Mirrors the
// MME's ConnectedSubscribers.
func (amf *AMF) ConnectedSubscribers() map[string]ConnectedSubscriber {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	out := make(map[string]ConnectedSubscriber)

	for _, ue := range amf.UEs {
		if !ue.supi.IsValid() || !ue.supi.IsIMSI() {
			continue
		}

		ue.mu.Lock()
		registered := ue.state == Registered

		radioName := ""
		if r := ue.active.Load(); r != nil {
			radioName = r.radioName
		}

		numSessions := len(ue.SmContextList)
		ue.mu.Unlock()

		if !registered {
			continue
		}

		out[ue.supi.IMSI()] = ConnectedSubscriber{
			RadioName:   radioName,
			NumSessions: numSessions,
			LastSeenAt:  ue.lastSeenTime(),
		}
	}

	return out
}
