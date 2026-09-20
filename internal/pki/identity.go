// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pki

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func NewNodeID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate node id: %w", err)
	}

	return id.String(), nil
}

func NormalizeNodeID(raw string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(raw))

	if id == "" {
		return "", fmt.Errorf("node id must not be empty")
	}

	if n, ok := LegacyNodeID(id); ok {
		return strconv.Itoa(n), nil
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", fmt.Errorf("node id %q is neither a UUID nor an integer in [%d, %d]", raw, MinNodeID, MaxNodeID)
	}

	if parsed.String() != id {
		return "", fmt.Errorf("node id %q is not in canonical UUID form", raw)
	}

	return id, nil
}

func LegacyNodeID(id string) (int, bool) {
	n, err := strconv.ParseUint(id, 10, 31)
	if err != nil {
		return 0, false
	}

	if strconv.FormatUint(n, 10) != id {
		return 0, false
	}

	v := int(n)
	if v < MinNodeID || v > MaxNodeID {
		return 0, false
	}

	return v, true
}

type NodeID string

func (n NodeID) MarshalJSON() ([]byte, error) {
	if _, ok := LegacyNodeID(string(n)); ok {
		return []byte(n), nil
	}

	return json.Marshal(string(n))
}

func (n *NodeID) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" {
		*n = ""

		return nil
	}

	if strings.HasPrefix(raw, `"`) {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}

		raw = s
	}

	if raw == "" || raw == "0" {
		*n = ""

		return nil
	}

	id, err := NormalizeNodeID(raw)
	if err != nil {
		return err
	}

	*n = NodeID(id)

	return nil
}
