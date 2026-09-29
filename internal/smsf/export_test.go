// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

func (s *SMSF) TrackedUEs() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.ues)
}

func (s *SMSF) Waiting(imsi string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.waiting[imsi]

	return ok
}
