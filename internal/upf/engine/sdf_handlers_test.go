package engine

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func newTestEngine() *SessionEngine {
	return &SessionEngine{
		sessions:          make(map[uint64]*Session),
		policyToSEIDs:     make(map[int64]map[uint64]struct{}),
		SdfIndexAllocator: NewSdfIndexAllocator(ebpf.MaxSdfFilters),
		filtersByKey:      make(map[string]uint32),
	}
}

func TestUpdateFilters_PropagatesNewIndexToExistingPDRs(t *testing.T) {
	eng := newTestEngine()

	// Create a session with policyID=42, no filter rules at creation time.
	sess := NewSession(100)
	sess.SetPolicyID(42)

	// Add uplink PDR (no UEIP) with FilterMapIndex=0 (no filter).
	sess.PutPDR(1, SPDRInfo{
		PdrID: 1,
		TeID:  10,
		PdrInfo: ebpf.PdrInfo{
			SEID:           100,
			PdrID:          1,
			FilterMapIndex: ebpf.NoFilterIndex,
		},
	})

	// Add downlink PDR (has UEIP) with FilterMapIndex=0.
	sess.PutPDR(2, SPDRInfo{
		PdrID: 2,
		UEIP:  netip.MustParseAddr("10.0.0.1"),
		PdrInfo: ebpf.PdrInfo{
			SEID:           100,
			PdrID:          2,
			FilterMapIndex: ebpf.NoFilterIndex,
		},
	})

	eng.mu.Lock()
	eng.sessions[100] = sess
	eng.registerPolicy(42, 100)
	eng.mu.Unlock()

	// Now add uplink rules to policy 42. This should propagate to the uplink PDR.
	err := eng.UpdateFilters(42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("UpdateFilters uplink: %v", err)
	}

	// Verify uplink PDR got the new filter index.
	uplinkPDR := sess.GetPDR(1)
	if uplinkPDR.PdrInfo.FilterMapIndex == ebpf.NoFilterIndex {
		t.Error("uplink PDR FilterMapIndex was not updated, still 0")
	}

	// Verify downlink PDR was NOT changed (we only updated uplink).
	downlinkPDR := sess.GetPDR(2)
	if downlinkPDR.PdrInfo.FilterMapIndex != ebpf.NoFilterIndex {
		t.Errorf("downlink PDR FilterMapIndex was changed unexpectedly: got %d", downlinkPDR.PdrInfo.FilterMapIndex)
	}

	// Now add downlink rules. This should propagate to the downlink PDR.
	err = eng.UpdateFilters(42, models.DirectionDownlink, []models.FilterRule{
		{Protocol: 6, PortLow: 443, PortHigh: 443, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("UpdateFilters downlink: %v", err)
	}

	downlinkPDR = sess.GetPDR(2)
	if downlinkPDR.PdrInfo.FilterMapIndex == ebpf.NoFilterIndex {
		t.Error("downlink PDR FilterMapIndex was not updated, still 0")
	}
}

func TestUpdateFilters_ExistingSlotDoesNotReapply(t *testing.T) {
	eng := newTestEngine()

	// Pre-populate a filter slot for policy 42 uplink.
	err := eng.UpdateFilters(42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("initial UpdateFilters: %v", err)
	}

	idx, ok := eng.GetFilterIndex(42, models.DirectionUplink)
	if !ok {
		t.Fatal("filter index not found after initial UpdateFilters")
	}

	// Create session that already has the correct filter index.
	sess := NewSession(200)
	sess.SetPolicyID(42)
	sess.PutPDR(1, SPDRInfo{
		PdrID: 1,
		TeID:  20,
		PdrInfo: ebpf.PdrInfo{
			SEID:           200,
			PdrID:          1,
			FilterMapIndex: idx,
		},
	})

	eng.mu.Lock()
	eng.sessions[200] = sess
	eng.registerPolicy(42, 200)
	eng.mu.Unlock()

	// Update the same filter slot with new rules. This is an in-place update
	// (not a new allocation), so no PDR propagation is needed.
	err = eng.UpdateFilters(42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 17, PortLow: 53, PortHigh: 53, Action: models.Deny},
	})
	if err != nil {
		t.Fatalf("second UpdateFilters: %v", err)
	}

	// The PDR's FilterMapIndex should remain the same (the BPF slot was updated in place).
	uplinkPDR := sess.GetPDR(1)
	if uplinkPDR.PdrInfo.FilterMapIndex != idx {
		t.Errorf("filter index changed unexpectedly: got %d, want %d", uplinkPDR.PdrInfo.FilterMapIndex, idx)
	}
}

func TestUpdateFilters_MultipleSessionsSamePolicy(t *testing.T) {
	eng := newTestEngine()

	// Two sessions, both with policy 10, both with unfiltered uplink PDRs.
	for _, seid := range []uint64{300, 301} {
		sess := NewSession(seid)
		sess.SetPolicyID(10)
		sess.PutPDR(1, SPDRInfo{
			PdrID: 1,
			TeID:  uint32(seid),
			PdrInfo: ebpf.PdrInfo{
				SEID:           seid,
				PdrID:          1,
				FilterMapIndex: ebpf.NoFilterIndex,
			},
		})

		eng.mu.Lock()
		eng.sessions[seid] = sess
		eng.registerPolicy(10, seid)
		eng.mu.Unlock()
	}

	err := eng.UpdateFilters(10, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("UpdateFilters: %v", err)
	}

	for _, seid := range []uint64{300, 301} {
		sess := eng.GetSession(seid)
		pdr := sess.GetPDR(1)

		if pdr.PdrInfo.FilterMapIndex == ebpf.NoFilterIndex {
			t.Errorf("session %d: uplink PDR FilterMapIndex was not updated", seid)
		}
	}
}

func TestDeleteSession_DeregistersFromPolicyIndex(t *testing.T) {
	eng := newTestEngine()

	sess := NewSession(400)
	sess.SetPolicyID(50)

	eng.mu.Lock()
	eng.sessions[400] = sess
	eng.registerPolicy(50, 400)
	eng.mu.Unlock()

	// Verify the reverse index has the entry.
	eng.mu.Lock()
	seids := eng.policyToSEIDs[50]
	eng.mu.Unlock()

	if len(seids) != 1 {
		t.Fatalf("expected 1 SEID in reverse index, got %d", len(seids))
	}

	// Simulate deletion.
	policyID := sess.PolicyID()

	eng.mu.Lock()
	delete(eng.sessions, 400)
	eng.deregisterPolicy(policyID, 400)
	eng.mu.Unlock()

	eng.mu.Lock()

	_, exists := eng.policyToSEIDs[50]

	eng.mu.Unlock()

	if exists {
		t.Error("reverse index entry not cleaned up after session deletion")
	}
}
