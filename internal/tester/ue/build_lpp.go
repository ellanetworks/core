// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/free5gc/aper"
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

	var gnssIDs []aper.Enumerated

	if opts.GNSSGPS {
		gnssIDs = append(gnssIDs, lpptype.GnssIDGps)
	}

	if opts.GNSSGLO {
		gnssIDs = append(gnssIDs, lpptype.GnssIDGlonass)
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

// DecodeLPPMessage decodes an APER-encoded LPP message from the LMF.
// Returns the transaction ID and the message body kind (lpptype.LPPMessageBodyC1Present*).
func DecodeLPPMessage(data []byte) (transactionID byte, bodyKind int, err error) {
	decoded, err := lpp.DecodeLPPMessage(data)
	if err != nil {
		return 0, 0, fmt.Errorf("decode LPP message: %w", err)
	}

	return decoded.TransactionID, decoded.BodyKind, nil
}
