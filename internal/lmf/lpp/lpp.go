// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

// LPP message types per 3GPP TS 36.355.
const (
	MsgTypeRequestLocationInformation  = 0x01
	MsgTypeProvideLocationCapabilities = 0x02
	MsgTypeProvideAssistanceData       = 0x03
	MsgTypeProvideLocationInformation  = 0x04
)

// PositioningMethod identifies the requested positioning method.
const (
	PosMethodGNSS     = 0x01
	PosMethodOTDOA    = 0x02
	PosMethodECID     = 0x03
	PosMethodAIDGNSS  = 0x04
	PosMethodAIDOTDOA = 0x05
	PosMethodAIDECID  = 0x06
)
