// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
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
			sdfRule.RemoteIP = prefix.Masked().Addr().As16()
			sdfRule.PrefixLen = uint8(prefix.Bits())
		}
	}

	sdfRule.PortLow = uint16(rule.PortLow)
	sdfRule.PortHigh = uint16(rule.PortHigh)

	return sdfRule
}

// The caller holds filterMu until it has applied the index: a slot released in
// between is reissued to the next policy.
func (conn *SessionEngine) resolveFilterIndexLocked(policyID string, direction models.Direction) uint32 {
	if policyID == "" {
		return ebpf.NoFilterIndex
	}

	key := fmt.Sprintf("%s:%s", policyID, direction.String())
	if idx, ok := conn.filtersByKey[key]; ok {
		return idx
	}

	return ebpf.NoFilterIndex
}

// UpdateFilters is an idempotent PUT-style operation for the sdf_filters BPF
// slot of a given (PolicyID, Direction) pair.
//
//   - Non-empty rules: allocate or update the BPF slot, propagate to PDRs.
//   - Empty rules: reset PDRs to NoFilterIndex, then zero and free the slot.
//
// Every path is safe to repeat: the reconciler retries unless this returns nil,
// so an error leaves the mapping in place for the retry to resume.
//
// filterMu is held for writing across the whole operation, propagation included:
// the allocator is LIFO, so a slot freed here is the next one handed out, and a
// session resolving it mid-propagation would be missed and left pointing at
// another policy's rules.
func (conn *SessionEngine) UpdateFilters(_ context.Context, policyID string, direction models.Direction, rules []models.FilterRule) error {
	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	key := fmt.Sprintf("%s:%s", policyID, direction.String())

	if len(rules) == 0 {
		return conn.releaseFilter(policyID, direction, key)
	}

	return conn.installFilter(policyID, direction, key, rules)
}

// installFilter writes the rules to the (PolicyID, Direction) slot, allocating
// one if the pair has none, and points the matching PDRs at it. Caller holds
// filterMu for writing.
func (conn *SessionEngine) installFilter(policyID string, direction models.Direction, key string, rules []models.FilterRule) error {
	sdfRules := make([]ebpf.SdfRule, 0, len(rules))
	for _, r := range rules {
		sdfRules = append(sdfRules, updateFiltersRule(r))
	}

	list := ebpf.SdfFilterList{NumRules: uint8(len(sdfRules))}
	copy(list.Rules[:len(sdfRules)], sdfRules)

	idx, existing := conn.filtersByKey[key]

	if !existing {
		allocated, err := conn.SdfIndexAllocator.Allocate()
		if err != nil {
			return fmt.Errorf("allocate sdf filter index: %w", err)
		}

		idx = allocated
	}

	if conn.BpfObjects != nil {
		if err := conn.BpfObjects.PutSdfFilterList(idx, list); err != nil {
			if !existing {
				conn.SdfIndexAllocator.Release(idx)
			}

			return fmt.Errorf("write sdf filter list: %w", err)
		}
	}

	conn.filtersByKey[key] = idx

	// Even when the slot existed: an earlier call may have written it and then
	// failed partway through propagation, and nothing else repairs that.
	return conn.propagateFilterIndex(policyID, direction, idx)
}

// releaseFilter clears the (PolicyID, Direction) slot off its PDRs and frees it.
// Caller holds filterMu for writing.
func (conn *SessionEngine) releaseFilter(policyID string, direction models.Direction, key string) error {
	idx, ok := conn.filtersByKey[key]
	if !ok {
		return nil
	}

	// Before the slot can be reissued: the allocator is LIFO, so the next
	// policy is handed this index.
	if err := conn.propagateFilterIndex(policyID, direction, ebpf.NoFilterIndex); err != nil {
		return err
	}

	// A zeroed list matches nothing, which is what a session still pointing at
	// a reissued slot should see.
	if conn.BpfObjects != nil {
		if err := conn.BpfObjects.DeleteSdfFilterList(idx); err != nil {
			return fmt.Errorf("clear sdf filter list: %w", err)
		}
	}

	delete(conn.filtersByKey, key)

	conn.SdfIndexAllocator.Release(idx)

	return nil
}

// propagateFilterIndex updates FilterMapIndex on all PDRs matching (policyID, direction).
//
// Every session is attempted: nothing else revisits one left pointing at an
// index about to be freed.
func (conn *SessionEngine) propagateFilterIndex(policyID string, direction models.Direction, idx uint32) error {
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

	var errs []error

	for _, seid := range seidList {
		session := conn.GetSession(seid)
		if session == nil {
			continue
		}

		if err := conn.applyFilterIndexToSession(session, isUplink, idx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// applyFilterIndexToSession updates FilterMapIndex on one session's matching
// PDRs under its operation lock, so it cannot interleave with a modify or delete
// of the same session.
func (conn *SessionEngine) applyFilterIndexToSession(session *Session, isUplink bool, idx uint32) error {
	session.opMu.Lock()
	defer session.opMu.Unlock()

	if session.deleted {
		return nil
	}

	var errs []error

	for pdrID, spdrInfo := range session.ListPDRs() {
		pdrIsUplink := !spdrInfo.UEIP.IsValid()
		if pdrIsUplink != isUplink {
			continue
		}

		spdrInfo.PdrInfo.FilterMapIndex = idx
		session.PutPDR(pdrID, spdrInfo)

		if conn.BpfObjects != nil {
			// One PDR's failure must not strand the rest on the old index.
			if err := applyPDR(spdrInfo, session, conn.BpfObjects); err != nil {
				errs = append(errs, fmt.Errorf("propagate filter index to PDR %d (SEID %d): %w", pdrID, session.SEID, err))
			}
		}
	}

	return errors.Join(errs...)
}
