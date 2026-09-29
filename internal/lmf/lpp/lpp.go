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

const (
	MethodGNSS = "gnss"
	MethodECID = "ecid"
)
