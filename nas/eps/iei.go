// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

// Information element identifiers shared by more than one EMM message. An
// optional IE's IEI is message-scoped in TS 24.301, but these elements keep the
// same IEI wherever they appear, so each is defined once here.
const (
	// gutiIEI is the GUTI IE (TS 24.301), assigned to the UE in ATTACH
	// ACCEPT and TRACKING AREA UPDATE ACCEPT.
	gutiIEI = 0x50
	// emmCauseIEI is the EMM cause IE (TS 24.301), carried in ATTACH
	// ACCEPT, TRACKING AREA UPDATE ACCEPT, and a network-originating DETACH REQUEST.
	emmCauseIEI = 0x53
	// epsNetworkFeatureSupportIEI is the EPS network feature support IE (TS 24.301),
	// carried in ATTACH ACCEPT and TRACKING AREA UPDATE ACCEPT.
	epsNetworkFeatureSupportIEI = 0x64
)
