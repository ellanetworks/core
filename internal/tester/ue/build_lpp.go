// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
)

// LPPCapabilitiesResponseOpts contains the parameters for building the UE's
// ProvideLocationCapabilities LPP response.
type LPPCapabilitiesResponseOpts struct {
	TransactionID byte
	GNSSGPS       bool
	GNSSGLO       bool
	GNSSBDT       bool
}

// BuildLPPCapabilitiesResponse creates an APER-encoded LPP ProvideCapabilities
// message (without NAS wrapper).
func BuildLPPCapabilitiesResponse(opts *LPPCapabilitiesResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, nil
	}

	var gnssIDs []lpptype.GNSSIDValue

	if opts.GNSSGPS {
		gnssIDs = append(gnssIDs, lpptype.GNSSIDGPS)
	}

	if opts.GNSSGLO {
		gnssIDs = append(gnssIDs, lpptype.GNSSIDGLONASS)
	}

	return lpp.EncodeProvideCapabilities(opts.TransactionID, gnssIDs)
}

// LPPLocationResponseOpts contains the parameters for building the UE's
// ProvideLocationInformation LPP response (UE-assisted GNSS fix).
type LPPLocationResponseOpts struct {
	TransactionID      byte
	Latitude           int32  // 1e-7 degrees
	Longitude          int32  // 1e-7 degrees
	Altitude           int32  // cm
	HorizontalAccuracy uint16 // meters
	VerticalAccuracy   uint16 // meters
	Timestamp          int64  // Unix milliseconds
}

// BuildLPPLocationResponse creates an APER-encoded LPP ProvideLocationInformation
// message (without NAS wrapper).
func BuildLPPLocationResponse(opts *LPPLocationResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, nil
	}

	return lpp.EncodeProvideLocationInformation(opts.TransactionID, opts.Latitude, opts.Longitude, opts.Altitude, uint32(opts.HorizontalAccuracy), uint32(opts.VerticalAccuracy))
}
