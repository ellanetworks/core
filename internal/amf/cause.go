// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

// 5GMM cause values (TS 24.501 §9.11.3.2).
const (
	GmmCause5GSServicesNotAllowed     uint8 = 7
	GmmCauseUEIdentityCannotBeDerived uint8 = 9
	GmmCauseTrackingAreaNotAllowed    uint8 = 12
	GmmCauseMACFailure                uint8 = 20
	GmmCauseSynchFailure              uint8 = 21
	GmmCauseNon5GAuthUnacceptable     uint8 = 26
	GmmCauseNgKSIAlreadyInUse         uint8 = 71
	GmmCausePayloadWasNotForwarded    uint8 = 90
	GmmCauseInvalidMandatoryInfo      uint8 = 96
	GmmCauseProtocolErrorUnspecified  uint8 = 111
)
