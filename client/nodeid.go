// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"encoding/json"
	"strings"
)

type NodeID string

func (n NodeID) String() string {
	return string(n)
}

func (n NodeID) MarshalJSON() ([]byte, error) {
	if isDecimal(string(n)) {
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

	*n = NodeID(strings.ToLower(strings.TrimSpace(raw)))

	return nil
}

func isDecimal(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
