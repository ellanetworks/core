// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

// BuildLocationInformation constructs an LPP ProvideLocationInformation message
// from a GNSS fix result. Latitude/longitude are in 1e-7 degrees, altitude in cm.
// hAcc and vAcc are horizontal/vertical accuracy in meters.
func BuildLocationInformation(transactionID byte, lat int32, lon int32, alt int32, hAcc, vAcc uint32) ([]byte, error) {
	return EncodeProvideLocationInformation(transactionID, lat, lon, alt, hAcc, vAcc)
}
