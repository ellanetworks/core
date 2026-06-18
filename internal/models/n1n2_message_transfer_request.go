// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type N1N2MessageTransferRequest struct {
	PduSessionID            uint8
	SNssai                  *Snssai
	BinaryDataN1Message     []byte
	BinaryDataN2Information []byte
}
