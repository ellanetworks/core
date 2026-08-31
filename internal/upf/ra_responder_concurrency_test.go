// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"net/netip"
	"sync"
	"testing"
)

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

func TestRAResponderRegisterUnregister(t *testing.T) {
	r := &RAResponder{
		sessions: make(map[uint32]*IPv6SessionContext),
		injectFD: -1,
	}

	sessionCtx := &IPv6SessionContext{
		DownlinkTEID: 0x1234,
		GnbN3Addr:    netip.MustParseAddr("10.0.0.1"),
		Prefix:       netip.MustParsePrefix("2001:db8:1::/64"),
		MTU:          1400,
		QFI:          9,
	}

	r.RegisterSession(0xBEEF, sessionCtx)

	if got := r.sessions[0xBEEF]; got != sessionCtx {
		t.Fatalf("sessions[0xBEEF] = %v, want the registered context", got)
	}

	replacement := &IPv6SessionContext{
		DownlinkTEID: 0x5678,
		GnbN3Addr:    netip.MustParseAddr("10.0.0.2"),
		Prefix:       netip.MustParsePrefix("2001:db8:2::/64"),
		MTU:          1400,
		QFI:          9,
	}

	r.RegisterSession(0xBEEF, replacement)

	if got := r.sessions[0xBEEF]; got != replacement {
		t.Fatalf("a re-registration left the stale context in place: %v", got)
	}

	if len(r.sessions) != 1 {
		t.Fatalf("a re-registration added a second entry: %d sessions", len(r.sessions))
	}

	r.UnregisterSession(0xBEEF)

	if _, ok := r.sessions[0xBEEF]; ok {
		t.Fatal("the session survived UnregisterSession")
	}

	r.UnregisterSession(0xBEEF)
}
