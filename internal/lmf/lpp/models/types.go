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

// GnssID represents a GNSS constellation identifier per TS 37.355 §6.4.1.
type GnssID int

const (
	GnssIDGps     GnssID = iota // 0
	GnssIDSbas                  // 1
	GnssIDQzss                  // 2
	GnssIDGalileo               // 3
	GnssIDGlonass               // 4
	GnssIDBds                   // 5
	GnssIDNavic                 // 6
)

func (id GnssID) String() string {
	switch id {
	case GnssIDGps:
		return "GPS"
	case GnssIDSbas:
		return "SBAS"
	case GnssIDQzss:
		return "QZSS"
	case GnssIDGalileo:
		return "Galileo"
	case GnssIDGlonass:
		return "GLONASS"
	case GnssIDBds:
		return "BDS"
	case GnssIDNavic:
		return "NavIC"
	default:
		return "unknown"
	}
}

// GNSSCapability indicates which GNSS constellations the UE supports.
type GNSSCapability struct {
	supported []GnssID
}

// AddSupported adds a GNSS constellation to the capability list.
func (c *GNSSCapability) AddSupported(id GnssID) {
	for _, existing := range c.supported {
		if existing == id {
			return
		}
	}

	c.supported = append(c.supported, id)
}

// Supported returns the list of supported GNSS constellations.
func (c *GNSSCapability) Supported() []GnssID {
	return c.supported
}

// Supports returns true if the given GNSS constellation is supported.
func (c *GNSSCapability) Supports(id GnssID) bool {
	for _, existing := range c.supported {
		if existing == id {
			return true
		}
	}

	return false
}

// ProvideAssistanceData is sent by LMF to UE with assistance data.
type ProvideAssistanceData struct {
	TransactionID      byte
	GNSSAssistanceData []byte // Raw ASN.1 encoded GNSS assistance data
}

// ProvideLocationInformation is sent by UE to LMF with the location fix.
//
// A target that cannot compute a position still answers, carrying an error
// cause and no locationEstimate (TS 37.355 §6.4.2, §6.5.2). HasEstimate says
// whether GNSSPositionResult holds a position the UE actually reported: without
// it the zero value is not a fix at (0, 0), it is the absence of one.
type ProvideLocationInformation struct {
	TransactionID      byte
	HasEstimate        bool
	FailureCause       string
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

// Abort is sent by either endpoint to abandon an ongoing procedure
// (TS 37.355 §5.5). Cause is the target's only account of why it stopped.
type Abort struct {
	TransactionID byte
	Cause         string
}
