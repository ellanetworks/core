// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

// handlePaging accepts a PAGING (TS 38.413 §8.5.1) without acting on it: the
// scenarios that care about paging consume the frame through WaitForMessage.
func handlePaging(_ *GnodeB, _ []byte) error {
	return nil
}
