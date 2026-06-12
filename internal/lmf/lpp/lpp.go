// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

// LPP message body kind values (matching lpptype.LPPMessageBodyC1Present*).
const (
	MsgRequestCapabilities        = 1
	MsgProvideCapabilities        = 2
	MsgRequestAssistanceData      = 3
	MsgProvideAssistanceData      = 4
	MsgRequestLocationInformation = 5
	MsgProvideLocationInformation = 6
)

// PositioningMethod identifies the requested positioning method.
const (
	PosMethodGNSS    = 0x01
	PosMethodOTDOA   = 0x02
	PosMethodECID    = 0x03
	PosMethodAIDGNSS = 0x04
)
