package core

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type Action int

const (
	Allow Action = iota
	Deny
)

// StringToAction takes an action string and returns an Action value
func StringToAction(a string) Action {
	if a == "deny" {
		return Deny
	}

	return Allow
}

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
		_, ipnet, err := net.ParseCIDR(rule.RemotePrefix)
		if err == nil {
			sdfRule.RemoteIP = binary.BigEndian.Uint32(ipnet.IP.To4())
			sdfRule.RemoteMask = binary.BigEndian.Uint32(ipnet.Mask)
		}
	}

	sdfRule.PortLow = uint16(rule.PortLow)
	sdfRule.PortHigh = uint16(rule.PortHigh)

	return sdfRule
}

// UpdateFilters allocates or refreshes a sdf_filters BPF array slot for the
// given (PolicyID, Direction) pair. It is idempotent: concurrent calls for the
// same key update the BPF slot in place.
func UpdateFilters(conn *PfcpConnection, policyID int64, direction string, rules []models.FilterRule) error {
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
				return fmt.Errorf("update sdf filter list: %w", err)
			}
		}

		return nil
	}

	idx, err := conn.SdfIndexAllocator.Allocate()
	if err != nil {
		return fmt.Errorf("allocate sdf filter index: %w", err)
	}

	if err := conn.BpfObjects.PutSdfFilterList(idx, list); err != nil {
		conn.SdfIndexAllocator.Release(idx)
		return fmt.Errorf("write sdf filter list: %w", err)
	}

	conn.filtersByKey[key] = idx

	return nil
}

// GetFilterIndex retrieves the BPF sdf_filters map index for a given (PolicyID, Direction) pair.
// Returns the index and true if found, or 0 and false if not allocated.
func GetFilterIndex(policyID int64, direction string) (uint32, bool) {
	conn := GetConnection()
	if conn == nil {
		return 0, false
	}

	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	key := fmt.Sprintf("%d:%s", policyID, direction)
	idx, ok := conn.filtersByKey[key]

	return idx, ok
}
