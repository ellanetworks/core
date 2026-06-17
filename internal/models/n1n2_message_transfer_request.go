// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

type N1N2MessageTransferRequest struct {
	PduSessionID            uint8
	SNssai                  *Snssai
	BinaryDataN1Message     []byte
	BinaryDataN2Information []byte
}
