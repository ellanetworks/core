// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
)

func TestNew(t *testing.T) {
	if newTestMME(t) == nil {
		t.Fatal("New returned nil")
	}
}
