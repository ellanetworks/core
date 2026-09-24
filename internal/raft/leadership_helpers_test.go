// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

func (m *Manager) leadershipEstablished() bool {
	m.leaderMu.Lock()
	defer m.leaderMu.Unlock()

	return m.term != nil && m.term.hooksStarted
}
