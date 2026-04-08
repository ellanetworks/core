package engine

import (
	"context"
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

// addSessionWithPDRs is a test helper that creates a session with one uplink
// and one downlink PDR, registers it in the engine, and returns the session.
func addSessionWithPDRs(t *testing.T, eng *SessionEngine, seid uint64, policyID int64) *Session {
	t.Helper()

	sess := NewSession(seid)
	sess.SetPolicyID(policyID)

	sess.PutPDR(1, SPDRInfo{
		PdrID: 1,
		TeID:  uint32(seid),
		PdrInfo: ebpf.PdrInfo{
			SEID:           seid,
			PdrID:          1,
			FilterMapIndex: ebpf.NoFilterIndex,
		},
	})

	sess.PutPDR(2, SPDRInfo{
		PdrID: 2,
		UEIP:  netip.MustParseAddr("10.0.0.1"),
		PdrInfo: ebpf.PdrInfo{
			SEID:           seid,
			PdrID:          2,
			FilterMapIndex: ebpf.NoFilterIndex,
		},
	})

	eng.mu.Lock()
	eng.sessions[seid] = sess
	eng.registerPolicy(policyID, seid)
	eng.mu.Unlock()

	return sess
}

func TestUpdateFilters_NewRulesPropagatesToPDRs(t *testing.T) {
	eng := newTestEngine()
	sess := addSessionWithPDRs(t, eng, 100, 42)

	// Add uplink rules — should propagate to uplink PDR only.
	err := eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("UpdateFilters uplink: %v", err)
	}

	uplinkPDR := sess.GetPDR(1)
	if uplinkPDR.PdrInfo.FilterMapIndex == ebpf.NoFilterIndex {
		t.Error("uplink PDR FilterMapIndex was not updated")
	}

	downlinkPDR := sess.GetPDR(2)
	if downlinkPDR.PdrInfo.FilterMapIndex != ebpf.NoFilterIndex {
		t.Errorf("downlink PDR FilterMapIndex was changed unexpectedly: got %d", downlinkPDR.PdrInfo.FilterMapIndex)
	}

	// Add downlink rules — should propagate to downlink PDR only.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionDownlink, []models.FilterRule{
		{Protocol: 6, PortLow: 443, PortHigh: 443, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("UpdateFilters downlink: %v", err)
	}

	downlinkPDR = sess.GetPDR(2)
	if downlinkPDR.PdrInfo.FilterMapIndex == ebpf.NoFilterIndex {
		t.Error("downlink PDR FilterMapIndex was not updated")
	}
}

func TestUpdateFilters_InPlaceUpdateKeepsSameSlot(t *testing.T) {
	eng := newTestEngine()

	// Allocate a slot.
	err := eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("initial UpdateFilters: %v", err)
	}

	idx := eng.resolveFilterIndex(42, "uplink")
	if idx == ebpf.NoFilterIndex {
		t.Fatal("filter index not found after initial UpdateFilters")
	}

	// Create session with the current filter index.
	sess := addSessionWithPDRs(t, eng, 200, 42)

	// Manually set the uplink PDR to use the allocated index.
	spdr := sess.GetPDR(1)
	spdr.PdrInfo.FilterMapIndex = idx
	sess.PutPDR(1, spdr)

	// Update the same slot with different rules — should be in-place.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 17, PortLow: 53, PortHigh: 53, Action: models.Deny},
	})
	if err != nil {
		t.Fatalf("second UpdateFilters: %v", err)
	}

	// The PDR's FilterMapIndex should remain the same (BPF slot updated in place).
	uplinkPDR := sess.GetPDR(1)
	if uplinkPDR.PdrInfo.FilterMapIndex != idx {
		t.Errorf("filter index changed unexpectedly: got %d, want %d", uplinkPDR.PdrInfo.FilterMapIndex, idx)
	}

	// The resolved index should still be the same.
	if eng.resolveFilterIndex(42, "uplink") != idx {
		t.Error("resolveFilterIndex returned different index after in-place update")
	}
}

func TestUpdateFilters_MultipleSessionsSamePolicy(t *testing.T) {
	eng := newTestEngine()

	for _, seid := range []uint64{300, 301} {
		addSessionWithPDRs(t, eng, seid, 10)
	}

	err := eng.UpdateFilters(context.Background(), 10, models.DirectionUplink, []models.FilterRule{
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

func TestUpdateFilters_EmptyRulesClearsFilter(t *testing.T) {
	eng := newTestEngine()
	sess := addSessionWithPDRs(t, eng, 500, 42)

	// Add uplink rules.
	err := eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Deny},
	})
	if err != nil {
		t.Fatalf("UpdateFilters with rules: %v", err)
	}

	pdr := sess.GetPDR(1)
	if pdr.PdrInfo.FilterMapIndex == ebpf.NoFilterIndex {
		t.Fatal("expected filter index to be set after UpdateFilters")
	}

	// Now update with empty rules — should clear the filter.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, nil)
	if err != nil {
		t.Fatalf("UpdateFilters with empty rules: %v", err)
	}

	pdr = sess.GetPDR(1)
	if pdr.PdrInfo.FilterMapIndex != ebpf.NoFilterIndex {
		t.Errorf("expected FilterMapIndex to be reset to NoFilterIndex, got %d", pdr.PdrInfo.FilterMapIndex)
	}

	if eng.resolveFilterIndex(42, "uplink") != ebpf.NoFilterIndex {
		t.Error("expected resolveFilterIndex to return NoFilterIndex after clearing")
	}
}

func TestUpdateFilters_EmptyRulesWhenNoSlotIsNoop(t *testing.T) {
	eng := newTestEngine()
	addSessionWithPDRs(t, eng, 600, 42)

	// Calling UpdateFilters with empty rules when no slot exists should be a no-op.
	err := eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, nil)
	if err != nil {
		t.Fatalf("UpdateFilters with empty rules (no existing slot): %v", err)
	}

	sess := eng.GetSession(600)
	pdr := sess.GetPDR(1)

	if pdr.PdrInfo.FilterMapIndex != ebpf.NoFilterIndex {
		t.Errorf("expected FilterMapIndex to remain NoFilterIndex, got %d", pdr.PdrInfo.FilterMapIndex)
	}
}

func TestUpdateFilters_ClearThenReaddAllocatesNewSlot(t *testing.T) {
	eng := newTestEngine()
	sess := addSessionWithPDRs(t, eng, 700, 42)

	// Add rules.
	err := eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("initial UpdateFilters: %v", err)
	}

	firstIdx := eng.resolveFilterIndex(42, "uplink")

	// Clear rules.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, nil)
	if err != nil {
		t.Fatalf("clear UpdateFilters: %v", err)
	}

	// Re-add rules — should allocate a new slot and propagate.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 17, PortLow: 53, PortHigh: 53, Action: models.Deny},
	})
	if err != nil {
		t.Fatalf("re-add UpdateFilters: %v", err)
	}

	newIdx := eng.resolveFilterIndex(42, "uplink")
	if newIdx == ebpf.NoFilterIndex {
		t.Fatal("expected a valid filter index after re-adding rules")
	}

	pdr := sess.GetPDR(1)
	if pdr.PdrInfo.FilterMapIndex != newIdx {
		t.Errorf("PDR FilterMapIndex = %d, want %d", pdr.PdrInfo.FilterMapIndex, newIdx)
	}

	// The released slot should be reusable, so the new index may or may not
	// equal the first one depending on allocator behavior. Just verify it's valid.
	_ = firstIdx
}

func TestUpdateFilters_OnlyAffectsMatchingDirection(t *testing.T) {
	eng := newTestEngine()
	sess := addSessionWithPDRs(t, eng, 800, 42)

	// Add uplink rules.
	err := eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Deny},
	})
	if err != nil {
		t.Fatalf("UpdateFilters uplink: %v", err)
	}

	// Add downlink rules.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionDownlink, []models.FilterRule{
		{Protocol: 6, PortLow: 443, PortHigh: 443, Action: models.Deny},
	})
	if err != nil {
		t.Fatalf("UpdateFilters downlink: %v", err)
	}

	uplinkIdx := sess.GetPDR(1).PdrInfo.FilterMapIndex
	downlinkIdx := sess.GetPDR(2).PdrInfo.FilterMapIndex

	if uplinkIdx == ebpf.NoFilterIndex || downlinkIdx == ebpf.NoFilterIndex {
		t.Fatal("both directions should have filter indices")
	}

	// Clear only uplink — downlink should remain.
	err = eng.UpdateFilters(context.Background(), 42, models.DirectionUplink, nil)
	if err != nil {
		t.Fatalf("clear uplink: %v", err)
	}

	if sess.GetPDR(1).PdrInfo.FilterMapIndex != ebpf.NoFilterIndex {
		t.Error("uplink PDR should have been cleared")
	}

	if sess.GetPDR(2).PdrInfo.FilterMapIndex != downlinkIdx {
		t.Error("downlink PDR should not have been affected")
	}
}

func TestUpdateFilters_NoSessionsForPolicy(t *testing.T) {
	eng := newTestEngine()

	// UpdateFilters with no sessions for the policy should succeed without error.
	err := eng.UpdateFilters(context.Background(), 99, models.DirectionUplink, []models.FilterRule{
		{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow},
	})
	if err != nil {
		t.Fatalf("UpdateFilters with no sessions: %v", err)
	}

	// The slot should still be allocated (ready for future sessions).
	if eng.resolveFilterIndex(99, "uplink") == ebpf.NoFilterIndex {
		t.Error("expected filter index to be allocated even with no sessions")
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
