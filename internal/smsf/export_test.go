// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import "context"

func (s *SMSF) TrackedUEs() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.ues)
}

func (s *SMSF) Waiting(imsi string) bool {
	_, err := s.store.GetSMSWaiting(context.Background(), imsi)

	return err == nil
}

func (s *SMSF) HoldAlertForTest(imsi string) func() {
	s.mu.Lock()
	s.alerting[imsi] = false
	s.mu.Unlock()

	return func() {
		if !s.alertSettled(imsi) {
			go s.alertUntilSettled(context.Background(), imsi)
		}
	}
}
