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

// resolveFilterIndex returns the BPF filter map index for a (policyID, direction) pair.
// Returns ebpf.NoFilterIndex if no filter is allocated.
func (conn *SessionEngine) resolveFilterIndex(policyID int64, direction string) uint32 {
	if policyID == 0 {
		return ebpf.NoFilterIndex
	}

	key := fmt.Sprintf("%d:%s", policyID, direction)

	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	if idx, ok := conn.filtersByKey[key]; ok {
		return idx
	}

	return ebpf.NoFilterIndex
}

// UpdateFilters is an idempotent PUT-style operation for the sdf_filters BPF
// slot of a given (PolicyID, Direction) pair.
//
//   - Non-empty rules: allocate or update the BPF slot, propagate to PDRs.
//   - Empty rules: deallocate the slot and reset PDRs to NoFilterIndex.
func (conn *SessionEngine) UpdateFilters(_ context.Context, policyID int64, direction models.Direction, rules []models.FilterRule) error {
	key := fmt.Sprintf("%d:%s", policyID, direction.String())

	// Empty rules: deallocate the existing slot (if any) and reset PDRs.
	if len(rules) == 0 {
		conn.filterMu.Lock()

		idx, ok := conn.filtersByKey[key]
		if !ok {
			conn.filterMu.Unlock()
			return nil
		}

		delete(conn.filtersByKey, key)
		conn.filterMu.Unlock()

		conn.SdfIndexAllocator.Release(idx)

		return conn.propagateFilterIndex(policyID, direction, ebpf.NoFilterIndex)
	}

	// Non-empty rules: build the BPF filter list.
	sdfRules := make([]ebpf.SdfRule, 0, len(rules))
	for _, r := range rules {
		sdfRules = append(sdfRules, updateFiltersRule(r))
	}

	list := ebpf.SdfFilterList{NumRules: uint8(len(sdfRules))}
	copy(list.Rules[:len(sdfRules)], sdfRules)

	conn.filterMu.Lock()

	// Existing slot: update in place, no propagation needed.
	if entry, ok := conn.filtersByKey[key]; ok {
		if conn.BpfObjects != nil {
			if err := conn.BpfObjects.PutSdfFilterList(entry, list); err != nil {
				conn.filterMu.Unlock()
				return fmt.Errorf("update sdf filter list: %w", err)
			}
		}

		conn.filterMu.Unlock()

		return nil
	}

	// New slot: allocate, write, and propagate to existing sessions.
	idx, err := conn.SdfIndexAllocator.Allocate()
	if err != nil {
		conn.filterMu.Unlock()
		return fmt.Errorf("allocate sdf filter index: %w", err)
	}

	if conn.BpfObjects != nil {
		if err := conn.BpfObjects.PutSdfFilterList(idx, list); err != nil {
			conn.SdfIndexAllocator.Release(idx)
			conn.filterMu.Unlock()

			return fmt.Errorf("write sdf filter list: %w", err)
		}
	}

	conn.filtersByKey[key] = idx
	conn.filterMu.Unlock()

	return conn.propagateFilterIndex(policyID, direction, idx)
}

// propagateFilterIndex updates FilterMapIndex on all PDRs matching (policyID, direction).
func (conn *SessionEngine) propagateFilterIndex(policyID int64, direction models.Direction, idx uint32) error {
	conn.mu.RLock()

	seids, ok := conn.policyToSEIDs[policyID]
	if !ok {
		conn.mu.RUnlock()
		return nil
	}

	seidList := make([]uint64, 0, len(seids))
	for seid := range seids {
		seidList = append(seidList, seid)
	}

	conn.mu.RUnlock()

	isUplink := direction == models.DirectionUplink

	for _, seid := range seidList {
		session := conn.GetSession(seid)
		if session == nil {
			continue
		}

		for pdrID, spdrInfo := range session.ListPDRs() {
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
