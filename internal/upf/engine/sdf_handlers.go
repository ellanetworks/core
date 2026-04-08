package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// updateFiltersRule converts a FilterRule to an internal Action for BPF operations
func updateFiltersRule(rule models.FilterRule) ebpf.SdfRule {
	sdfRule := ebpf.SdfRule{
		Protocol: ebpf.SdfProtoAny,
		Action:   ebpf.SdfActionAllow,
	}
	if rule.Protocol != 0 {
		sdfRule.Protocol = uint8(rule.Protocol)
	}

	if rule.Action == models.Deny {
		sdfRule.Action = ebpf.SdfActionDeny
	}

	if rule.RemotePrefix != "" {
		prefix, err := netip.ParsePrefix(rule.RemotePrefix)
		if err == nil {
			addr4 := prefix.Masked().Addr().As4()
			sdfRule.RemoteIP = binary.BigEndian.Uint32(addr4[:])
			bits := prefix.Bits()

			mask := uint32(0)
			if bits > 0 {
				mask = ^uint32(0) << (32 - bits)
			}

			sdfRule.RemoteMask = mask
		}
	}

	sdfRule.PortLow = uint16(rule.PortLow)
	sdfRule.PortHigh = uint16(rule.PortHigh)

	return sdfRule
}

// updateFiltersOnConn allocates or refreshes a sdf_filters BPF array slot for the
// given (PolicyID, Direction) pair. It is idempotent: concurrent calls for the
// same key update the BPF slot in place.
// Returns the BPF slot index and whether a new slot was allocated.
func updateFiltersOnConn(_ context.Context, conn *SessionEngine, policyID int64, direction string, rules []models.FilterRule) (uint32, bool, error) {
	key := fmt.Sprintf("%d:%s", policyID, direction)

	sdfRules := make([]ebpf.SdfRule, 0, len(rules))
	for _, r := range rules {
		sdfRules = append(sdfRules, updateFiltersRule(r))
	}

	list := ebpf.SdfFilterList{NumRules: uint8(len(sdfRules))}
	copy(list.Rules[:len(sdfRules)], sdfRules)

	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	if entry, ok := conn.filtersByKey[key]; ok {
		// Update BPF slot in place (skip if BPF objects not available - test mode)
		if conn.BpfObjects != nil {
			if err := conn.BpfObjects.PutSdfFilterList(entry, list); err != nil {
				return 0, false, fmt.Errorf("update sdf filter list: %w", err)
			}
		}

		return entry, false, nil
	}

	idx, err := conn.SdfIndexAllocator.Allocate()
	if err != nil {
		return 0, false, fmt.Errorf("allocate sdf filter index: %w", err)
	}

	if conn.BpfObjects != nil {
		if err := conn.BpfObjects.PutSdfFilterList(idx, list); err != nil {
			conn.SdfIndexAllocator.Release(idx)
			return 0, false, fmt.Errorf("write sdf filter list: %w", err)
		}
	}

	conn.filtersByKey[key] = idx

	return idx, true, nil
}

// UpdateFilters allocates or refreshes a sdf_filters BPF array slot for the
// given (PolicyID, Direction) pair. When a new slot is allocated, existing
// sessions using this policy have their PDRs updated to point to the new slot.
func (conn *SessionEngine) UpdateFilters(ctx context.Context, policyID int64, direction models.Direction, rules []models.FilterRule) error {
	idx, isNew, err := updateFiltersOnConn(ctx, conn, policyID, direction.String(), rules)
	if err != nil {
		return err
	}

	if !isNew {
		return nil
	}

	// A new filter slot was allocated (first rules for this policy+direction).
	// Propagate the index to all existing sessions that reference this policy.
	conn.mu.Lock()

	seids, ok := conn.policyToSEIDs[policyID]
	if !ok {
		conn.mu.Unlock()
		return nil
	}

	// Copy the SEID set so we can release conn.mu before touching sessions.
	seidList := make([]uint64, 0, len(seids))
	for seid := range seids {
		seidList = append(seidList, seid)
	}

	conn.mu.Unlock()

	isUplink := direction == models.DirectionUplink

	for _, seid := range seidList {
		session := conn.GetSession(seid)
		if session == nil {
			continue
		}

		for pdrID, spdrInfo := range session.ListPDRs() {
			// Match direction: uplink PDRs have no UEIP, downlink PDRs have UEIP.
			pdrIsUplink := !spdrInfo.UEIP.IsValid()
			if pdrIsUplink != isUplink {
				continue
			}

			spdrInfo.PdrInfo.FilterMapIndex = idx
			session.PutPDR(pdrID, spdrInfo)

			if conn.BpfObjects != nil {
				if err := applyPDR(spdrInfo, conn.BpfObjects); err != nil {
					return fmt.Errorf("propagate filter index to PDR %d (SEID %d): %w", pdrID, seid, err)
				}
			}
		}
	}

	return nil
}

// GetFilterIndex retrieves the BPF sdf_filters map index for a given (PolicyID, Direction) pair.
// Returns the index and true if found, or 0 and false if not allocated.
func (conn *SessionEngine) GetFilterIndex(policyID int64, direction models.Direction) (uint32, bool) {
	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	key := fmt.Sprintf("%d:%s", policyID, direction.String())
	idx, ok := conn.filtersByKey[key]

	return idx, ok
}
