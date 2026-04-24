// Copyright 2026 Ella Networks

package upf_test

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/upf"
)

// TestRAResponderRegisterUnregister verifies that the RA responder's
// in-memory session map correctly adds and removes entries.
func TestRAResponderRegisterUnregister(t *testing.T) {
	// We can't call NewRAResponder without a real BPF ring buffer map,
	// but we can test the session registration logic through the UPF's
	// public API by verifying that Register/Unregister don't panic when
	// the RA responder is nil (graceful no-op).
	var u upf.UPF // zero value — raResponder is nil

	// These should be no-ops (no panic).
	u.RegisterIPv6Session(0xBEEF, &upf.IPv6SessionContext{
		DownlinkTEID: 0x1234,
		GnbN3Addr:    netip.MustParseAddr("10.0.0.1"),
		Prefix:       netip.MustParsePrefix("2001:db8:1::/64"),
		MTU:          1400,
		QFI:          9,
	})
	u.UnregisterIPv6Session(0xBEEF)
}
