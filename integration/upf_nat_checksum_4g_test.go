// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import "testing"

// TestIntegration4GUPFNATChecksum verifies post-NAT L4 checksums for 4G UE
// traffic. See runNATChecksumSuite for the harness.
func TestIntegration4GUPFNATChecksum(t *testing.T) {
	runNATChecksumSuite(t, "s1enb/nat_checksum")
}
