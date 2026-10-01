// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"fmt"
	"strings"
)

type EnumNames []string

func (n EnumNames) Has(v uint32) bool {
	return int(v) < len(n) && n[v] != ""
}

func (n EnumNames) Name(typeName string, v uint32) string {
	if n.Has(v) {
		return n[v]
	}

	return fmt.Sprintf("%s(%d)", typeName, v)
}

func BitNames(v uint32, names ...string) string {
	if v == 0 {
		return "0"
	}

	var parts []string

	for i, name := range names {
		if bit := uint32(1) << i; v&bit != 0 {
			parts = append(parts, name)
			v &^= bit
		}
	}

	if v != 0 {
		parts = append(parts, fmt.Sprintf("%#x", v))
	}

	return strings.Join(parts, "|")
}
