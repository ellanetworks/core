// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestTEIDPoolCoversEverySession(t *testing.T) {
	for _, sessions := range []uint32{1, ebpf.DefaultMaxSessions, FTEIDPool, 100000, ebpf.MaxSessionsLimit} {
		if got := teidPoolSize(sessions); got < sessions || got < FTEIDPool {
			t.Errorf("%d sessions: TEID pool %d, want at least %d and at least %d", sessions, got, sessions, FTEIDPool)
		}
	}
}
