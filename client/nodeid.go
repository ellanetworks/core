// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	minLegacyNodeID = 1
	maxLegacyNodeID = 63
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
	n, err := strconv.ParseUint(s, 10, 31)
	if err != nil {
		return false
	}

	if strconv.FormatUint(n, 10) != s {
		return false
	}

	return n >= minLegacyNodeID && n <= maxLegacyNodeID
}
