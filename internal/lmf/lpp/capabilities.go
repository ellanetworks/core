// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

// BuildRequestCapabilities constructs an LPP RequestCapabilities message
// asking the UE to provide its A-GNSS capabilities.
func BuildRequestCapabilities(transactionID, sequenceNumber byte) ([]byte, error) {
	return EncodeRequestCapabilities(transactionID, sequenceNumber)
}
