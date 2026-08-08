// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

// TransferPendingForTest reports whether a move is recorded but not committed.
// Caller holds Mutex.
func (smContext *SMContext) TransferPendingForTest() bool { return smContext.pending != nil }
