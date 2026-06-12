// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// RequestLocationInformation is sent by LMF to UE to request a location fix.
type RequestLocationInformation struct {
	TransactionID     byte
	PositioningMethod uint8
	NumberOfSVs       uint8
}

// ProvideLocationCapabilities is sent by UE to LMF in response.
type ProvideLocationCapabilities struct {
	TransactionID  byte
	GNSSCapability GNSSCapability
}

// GNSSCapability indicates which GNSS constellations the UE supports.
type GNSSCapability struct {
	GPS  bool
	GLO  bool
	BDT  bool
	QZS  bool
	SBS  bool
	IRN  bool
	ESAT bool
}

// ProvideAssistanceData is sent by LMF to UE with assistance data.
type ProvideAssistanceData struct {
	TransactionID      byte
	GNSSAssistanceData []byte // Raw ASN.1 encoded GNSS assistance data
}

// ProvideLocationInformation is sent by UE to LMF with the location fix.
type ProvideLocationInformation struct {
	TransactionID      byte
	GNSSPositionResult GNSSPositionResult
}

// GNSSPositionResult contains the GNSS-derived location.
type GNSSPositionResult struct {
	Latitude           int32  // in 1e-7 degrees
	Longitude          int32  // in 1e-7 degrees
	Altitude           int32  // in cm (WGS84 ellipsoid)
	HorizontalAccuracy uint32 // in meters
	VerticalAccuracy   uint32 // in meters
	Timestamp          int64  // Unix timestamp in ms
}
