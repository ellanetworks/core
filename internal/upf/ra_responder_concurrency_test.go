// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"net/netip"
	"sync"
	"testing"
)

// TestRAResponderConcurrentRegisterUnregister hammers Register/Unregister
// (including handover-style re-register for the same TEID) from many goroutines.
// Under -race it guards the r.mu discipline over the sessions map. vethBpf is
// nil, so the tunnel-map writes are guarded out — those need privileged veth to
// exercise.
func TestRAResponderConcurrentRegisterUnregister(t *testing.T) {
	r := &RAResponder{
		sessions: make(map[uint32]*IPv6SessionContext),
		injectFD: -1,
	}

	ctx := func() *IPv6SessionContext {
		return &IPv6SessionContext{
			DownlinkTEID: 0x1234,
			GnbN3Addr:    netip.MustParseAddr("10.0.0.1"),
			Prefix:       netip.MustParsePrefix("2001:db8:1::/64"),
			MTU:          1400,
			QFI:          9,
		}
	}

	var wg sync.WaitGroup

	for i := range 50 {
		teid := uint32(i)

		wg.Add(3)

		go func() { defer wg.Done(); r.RegisterSession(teid, ctx()) }()
		go func() { defer wg.Done(); r.RegisterSession(teid, ctx()) }()
		go func() { defer wg.Done(); r.UnregisterSession(teid) }()
	}

	wg.Wait()
}
