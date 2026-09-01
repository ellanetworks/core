// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

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
	s.write(imsi, radioID, radioName, at, true)
}

func (s *lastSeenStore) refresh(imsi string, radioID, radioName string, at time.Time) {
	s.write(imsi, radioID, radioName, at, false)
}

func (s *lastSeenStore) write(imsi string, radioID, radioName string, at time.Time, create bool) {
	if imsi == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, known := s.entries[imsi]
	if !known && !create {
		return
	}

	if s.entries == nil {
		s.entries = make(map[string]LastSeen)
	}

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

func (m *MME) LastSeen(imsi string) (LastSeen, bool) {
	entry, ok := m.lastSeen.get(imsi)
	if !ok {
		return entry, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entry.RadioName = m.resolveRadioNameLocked(entry)

	return entry, true
}

func (m *MME) LastSeenAll() map[string]LastSeen {
	entries := m.lastSeen.all()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for imsi, entry := range entries {
		entry.RadioName = m.resolveRadioNameLocked(entry)
		entries[imsi] = entry
	}

	return entries
}

func (m *MME) ForgetSubscriber(imsi string) {
	m.lastSeen.forget(imsi)
}

func (m *MME) resolveRadioNameLocked(entry LastSeen) string {
	if entry.RadioID == "" {
		return entry.RadioName
	}

	if radio, ok := m.reg.ClaimedBy(entry.RadioID); ok && radio.name != "" {
		return radio.name
	}

	return entry.RadioName
}
