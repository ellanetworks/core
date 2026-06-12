// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import "github.com/free5gc/aper"

// BuildRequestCapabilities constructs an LPP RequestCapabilities message
// asking the UE to provide its A-GNSS capabilities.
func BuildRequestCapabilities(transactionID byte) ([]byte, error) {
	return EncodeRequestCapabilities(transactionID)
}

// BuildProvideCapabilities constructs an LPP ProvideCapabilities response
// indicating the UE supports the given GNSS constellations.
// gnssIDs is a variadic list of GNSS constellation IDs (e.g., lpptype.GnssIDGps,
// lpptype.GnssIDGalileo, lpptype.GnssIDGlonass). See TS 37.355 §6.4.2.2.
func BuildProvideCapabilities(transactionID byte, gnssIDs ...aper.Enumerated) ([]byte, error) {
	enumerated := make([]aper.Enumerated, len(gnssIDs))
	copy(enumerated, gnssIDs)

	return EncodeProvideCapabilities(transactionID, enumerated)
}
