// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import "github.com/ellanetworks/core/internal/models"

func FarDestinationsForTest(fars []models.FAR) map[uint32]models.Interface {
	return farDestinations(fars)
}

func (conn *SessionEngine) NetworkInstanceIDForTest(name string) (uint8, error) {
	return conn.networkInstanceID(name)
}

func (conn *SessionEngine) AcquireDataNetworkTagForTest(id string) (uint16, error) {
	return conn.acquireDataNetworkTag(id)
}

func (conn *SessionEngine) ReleaseDataNetworkTagForTest(id string) {
	conn.releaseDataNetworkTag(id)
}

func (s *Session) NetworkInstanceForTest() uint8 {
	return s.networkInstance()
}
