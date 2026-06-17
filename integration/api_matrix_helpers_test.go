// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"fmt"
	"strings"
	"testing"
)

// apiMatrixName prefixes resource names so they're identifiable as matrix
// state and so a collision from a leaked prior run surfaces immediately.
func apiMatrixName(resource string) string {
	return "apimat-" + resource
}

func assertNotFound(t *testing.T, err error, what string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s lookup to fail with not-found, got nil", what)
	}

	msg := err.Error()
	Assert(t, strings.Contains(msg, "404") || strings.Contains(strings.ToLower(msg), "not found"), fmt.Sprintf("expected %s lookup to fail with not-found, got: %v", what, err))
}
