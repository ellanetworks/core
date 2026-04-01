package core

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/smf"
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

// UpdateFilterRule is a DTO carrying the fields needed to build a BPF sdf_rule.
type UpdateFilterRule struct {
	RemotePrefix string // CIDR notation; "" = any
	Protocol     int32  // 0 = any (maps to SdfProtoAny)
	PortLow      int32
	PortHigh     int32
	Action       Action
}

// UpdateFiltersRequest is the input to UpdateFilters.
type UpdateFiltersRequest struct {
	PolicyID  int64
	Direction smf.Direction
	Rules     []UpdateFilterRule
}

// UpdateFiltersResponse is the output of UpdateFilters.
type UpdateFiltersResponse struct {
	FilterMapIndex uint32
}

// UpdateFilters allocates or refreshes a sdf_filters BPF array slot for the
// given (PolicyID, Direction) pair. It is idempotent: concurrent calls for the
// same key return the same index and update the BPF slot in place.
func UpdateFilters(conn *PfcpConnection, req UpdateFiltersRequest) (*UpdateFiltersResponse, error) {
	key := fmt.Sprintf("%d:%s", req.PolicyID, req.Direction)

	sdfRules := resolveSdfRules(req.Rules)
	list := ebpf.SdfFilterList{NumRules: uint8(len(sdfRules))}
	copy(list.Rules[:len(sdfRules)], sdfRules)

	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	if entry, ok := conn.filtersByKey[key]; ok {
		// Update BPF slot in place; increment refcount.
		if err := conn.BpfObjects.PutSdfFilterList(entry.index, list); err != nil {
			return nil, fmt.Errorf("update sdf filter list: %w", err)
		}

		entry.refcount++

		return &UpdateFiltersResponse{FilterMapIndex: entry.index}, nil
	}

	// Allocate a new slot.
	idx, err := conn.SdfIndexAllocator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("allocate sdf filter index: %w", err)
	}

	if err := conn.BpfObjects.PutSdfFilterList(idx, list); err != nil {
		conn.SdfIndexAllocator.Release(idx)
		return nil, fmt.Errorf("write sdf filter list: %w", err)
	}

	conn.filtersByKey[key] = &filterEntry{index: idx, refcount: 1}

	return &UpdateFiltersResponse{FilterMapIndex: idx}, nil
}

// ReleaseFilter decrements the refcount for the given index and frees the
// BPF slot when it reaches zero.
func ReleaseFilter(conn *PfcpConnection, index uint32) error {
	if index == ebpf.NoFilterIndex {
		return nil
	}

	conn.filterMu.Lock()
	defer conn.filterMu.Unlock()

	for key, entry := range conn.filtersByKey {
		if entry.index != index {
			continue
		}

		entry.refcount--
		if entry.refcount <= 0 {
			if err := conn.BpfObjects.DeleteSdfFilterList(index); err != nil {
				return fmt.Errorf("zero sdf filter list: %w", err)
			}

			conn.SdfIndexAllocator.Release(index)
			delete(conn.filtersByKey, key)
		}

		return nil
	}

	return nil // index not found; no-op
}

func resolveSdfRules(rules []UpdateFilterRule) []ebpf.SdfRule {
	out := make([]ebpf.SdfRule, 0, len(rules))
	for _, r := range rules {
		rule := ebpf.SdfRule{
			Protocol: ebpf.SdfProtoAny,
			Action:   ebpf.SdfActionAllow,
		}
		if r.Protocol != 0 {
			rule.Protocol = uint8(r.Protocol)
		}

		if r.Action == Deny {
			rule.Action = ebpf.SdfActionDeny
		}

		if r.RemotePrefix != "" {
			_, ipnet, err := net.ParseCIDR(r.RemotePrefix)
			if err == nil {
				rule.RemoteIP = binary.BigEndian.Uint32(ipnet.IP.To4())
				rule.RemoteMask = binary.BigEndian.Uint32(ipnet.Mask)
			}
		}

		rule.PortLow = uint16(r.PortLow)
		rule.PortHigh = uint16(r.PortHigh)
		out = append(out, rule)
	}

	return out
}
