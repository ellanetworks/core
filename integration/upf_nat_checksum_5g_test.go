// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import "testing"

// TestIntegration5GUPFNATChecksum verifies post-NAT L4 checksums for 5G UE
// traffic. See runNATChecksumSuite for the harness.
func TestIntegration5GUPFNATChecksum(t *testing.T) {
	runNATChecksumSuite(t, "gnb/nat_checksum")
}
