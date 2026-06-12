// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

// TS 23.032 geographic coordinate encoding constants.
//
// The 3GPP TS 23.032 shape encoding uses fixed-point integers to represent
// latitude, longitude, altitude, and uncertainty. The resolution of each field
// is determined by a power-of-two range:
//
//   - Latitude:  23 bits, range 0..2^23-1,      resolution 90 / 2^23 degrees
//   - Longitude: 24 bits, range 0..2^24-1,      resolution 360 / 2^24 degrees
//   - Altitude:  15 bits, range 0..2^15-1,      resolution 1 metre
//   - Uncertainty: 7 bits, range 0..127,         resolution C*((1+x)^k - 1)
//
// Latitude and longitude are stored as 1e-7-degree integers in the public API.
// Altitude is stored in centimetres and converted to metres for the wire format.
const (
	// Latitude encoding: 2^23 range (23 bits).
	latitudeResolution = 8388608     // 2^23
	maxDegreesLatitude = 8388607     // 2^23 - 1
	maxLatitudeE7      = 900_000_000 // 90 degrees in 1e-7 units

	// Longitude encoding: 2^24 range (24 bits), stored as unsigned offset.
	longitudeResolution = 16777216      // 2^24
	maxDegreesLongitude = 16777215      // 2^24 - 1
	longitudeOffset     = 8388608       // 2^23 — bias added to signed value
	maxLongitudeE7      = 3_600_000_000 // 360 degrees in 1e-7 units

	// Altitude encoding: 2^15 range (15 bits), in metres.
	maxAltitude         = 32767 // 2^15 - 1
	centimetresPerMetre = 100

	// Uncertainty encoding (TS 23.032 §7.3.2):
	//   r = C * ((1+x)^k - 1), where C = 10, x = 0.1, k = 0..127.
	uncertaintyConstantC = 10.0
	uncertaintyFactorX   = 0.1
	uncertaintyBase      = 1.1 // 1 + x
	maxUncertaintyCode   = 127

	// Default confidence value for uncertainty ellipses (percent).
	// TS 23.032 recommends 67% as a typical confidence level.
	defaultConfidence = 67
)
