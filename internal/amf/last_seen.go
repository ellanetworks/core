// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"time"
)

type LastSeen struct {
	RadioID   string
	RadioName string
	At        time.Time
}

type lastSeenStore struct {
	mu      sync.RWMutex
	entries map[string]LastSeen
}

func (s *lastSeenStore) record(imsi string, radioID, radioName string, at time.Time) {
	if imsi == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.entries == nil {
		s.entries = make(map[string]LastSeen)
	}

	entry := s.entries[imsi]

	if radioID != "" || radioName != "" {
		entry.RadioID, entry.RadioName = radioID, radioName
	}

	if at.After(entry.At) {
		entry.At = at
	}

	s.entries[imsi] = entry
}

func (s *lastSeenStore) get(imsi string) (LastSeen, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[imsi]

	return entry, ok
}

func (s *lastSeenStore) all() map[string]LastSeen {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]LastSeen, len(s.entries))
	for imsi, entry := range s.entries {
		out[imsi] = entry
	}

	return out
}

func (s *lastSeenStore) forget(imsi string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, imsi)
}

func (a *AMF) LastSeen(imsi string) (LastSeen, bool) {
	entry, ok := a.lastSeen.get(imsi)
	if !ok {
		return entry, false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	entry.RadioName = a.resolveRadioNameLocked(entry)

	return entry, true
}

func (a *AMF) LastSeenAll() map[string]LastSeen {
	entries := a.lastSeen.all()

	a.mu.RLock()
	defer a.mu.RUnlock()

	for imsi, entry := range entries {
		entry.RadioName = a.resolveRadioNameLocked(entry)
		entries[imsi] = entry
	}

	return entries
}

func (a *AMF) ForgetSubscriber(imsi string) {
	a.lastSeen.forget(imsi)
}

func (a *AMF) resolveRadioNameLocked(entry LastSeen) string {
	if entry.RadioID == "" {
		return entry.RadioName
	}

	if radio, ok := a.reg.ClaimedBy(entry.RadioID); ok && radio.name != "" {
		return radio.name
	}

	return entry.RadioName
}
