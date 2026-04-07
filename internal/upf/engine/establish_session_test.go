// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"net/netip"
	"testing"
)

func TestCreatedPDRsToResponse_UplinkPDR(t *testing.T) {
	n3Addr := netip.MustParseAddr("10.3.0.2")
	pdrs := []SPDRInfo{
		{PdrID: 1, TeID: 42, Allocated: true},
	}

	result := createdPDRsToResponse(pdrs, n3Addr)

	if len(result) != 1 {
		t.Fatalf("expected 1 CreatedPDR, got %d", len(result))
	}

	if result[0].PDRID != 1 {
		t.Errorf("PDRID: got %d, want 1", result[0].PDRID)
	}

	if result[0].TEID != 42 {
		t.Errorf("TEID: got %d, want 42", result[0].TEID)
	}

	if result[0].N3IP != n3Addr {
		t.Errorf("N3IP: got %v, want %v", result[0].N3IP, n3Addr)
	}
}

func TestCreatedPDRsToResponse_DownlinkPDRExcluded(t *testing.T) {
	n3Addr := netip.MustParseAddr("10.3.0.2")
	pdrs := []SPDRInfo{
		{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.1"), Allocated: true},
	}

	result := createdPDRsToResponse(pdrs, n3Addr)

	if len(result) != 0 {
		t.Fatalf("expected 0 CreatedPDRs for downlink-only input, got %d", len(result))
	}
}

func TestCreatedPDRsToResponse_SkipsUnallocated(t *testing.T) {
	n3Addr := netip.MustParseAddr("10.3.0.2")
	pdrs := []SPDRInfo{
		{PdrID: 1, TeID: 42, Allocated: false},
		{PdrID: 2, TeID: 43, Allocated: true},
	}

	result := createdPDRsToResponse(pdrs, n3Addr)

	if len(result) != 1 {
		t.Fatalf("expected 1 CreatedPDR, got %d", len(result))
	}

	if result[0].PDRID != 2 {
		t.Errorf("PDRID: got %d, want 2", result[0].PDRID)
	}
}

func TestCreatedPDRsToResponse_MixedPDRs(t *testing.T) {
	n3Addr := netip.MustParseAddr("10.3.0.2")
	pdrs := []SPDRInfo{
		{PdrID: 1, TeID: 100, Allocated: true},
		{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.5"), Allocated: true},
		{PdrID: 3, TeID: 200, Allocated: false},
		{PdrID: 4, UEIP: netip.MustParseAddr("2001:db8::1"), Allocated: true},
	}

	result := createdPDRsToResponse(pdrs, n3Addr)

	// Only PDR 1 should be in response: uplink with TEID, allocated.
	// PDR 2 (downlink IPv4) and PDR 4 (downlink IPv6) are excluded.
	// PDR 3 is unallocated.
	if len(result) != 1 {
		t.Fatalf("expected 1 CreatedPDR, got %d", len(result))
	}

	if result[0].TEID != 100 || result[0].N3IP != n3Addr {
		t.Errorf("PDR 1: got TEID=%d N3IP=%v, want TEID=100 N3IP=%v", result[0].TEID, result[0].N3IP, n3Addr)
	}
}
