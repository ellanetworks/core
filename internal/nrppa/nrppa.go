// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nrppa implements a real 3GPP TS 38.455 NRPPa (NR Positioning Protocol
// A) ASN.1 codec for the E-CID Measurement Initiation procedure, using the
// aligned-PER library github.com/free5gc/aper over the hand-written, aper-tagged
// nrppatype structs.
//
// NRPPa is transported as an octet string inside NGAP UE-associated transport
// messages (TS 38.413 §8.10) between the LMF and the RAN. This package provides
// the low-level Encoder/Decoder plus high-level helpers that build and parse the
// three E-CID Measurement Initiation messages without exposing raw aper types to
// callers.
//
// Request/response correlation for the MVP is by LMF-UE-Measurement-ID (the
// LMF-assigned measurement id), echoed by the RAN. The NRPPa transaction id is
// carried in the elementary-procedure envelope and echoed for completeness.
package nrppa

import (
	"github.com/ellanetworks/core/internal/nrppa/nrppatype"
	"github.com/free5gc/aper"
)

// Encoder serialises an NRPPa-PDU to aligned-PER bytes. NRPPa-PDU is an
// extensible CHOICE of three root alternatives (initiating / successful /
// unsuccessful), hence the top-level params "valueExt,valueLB:0,valueUB:2".
func Encoder(pdu nrppatype.NRPPaPDU) ([]byte, error) {
	return aper.MarshalWithParams(pdu, "valueExt,valueLB:0,valueUB:2")
}

// Decoder parses aligned-PER bytes into an NRPPa-PDU.
func Decoder(b []byte) (*nrppatype.NRPPaPDU, error) {
	pdu := &nrppatype.NRPPaPDU{}

	err := aper.UnmarshalWithParams(b, pdu, "valueExt,valueLB:0,valueUB:2")
	if err != nil {
		return nil, err
	}

	return pdu, nil
}

// =====================================================================
// Plain caller-facing structs (no raw aper types leak to callers).
// =====================================================================

// MessageKind discriminates a decoded NRPPa E-CID PDU.
type MessageKind int

const (
	KindUnknown MessageKind = iota
	KindECIDMeasurementInitiationRequest
	KindECIDMeasurementInitiationResponse
	KindECIDMeasurementInitiationFailure
)

// MeasurementQuantityValue enumerates the E-CID measurement quantities
// (TS 38.455 MeasurementQuantitiesValue, root values only).
type MeasurementQuantityValue int

const (
	MeasCellID MeasurementQuantityValue = iota
	MeasAngleOfArrival
	MeasTimingAdvanceType1
	MeasTimingAdvanceType2
	MeasRSRP
	MeasRSRQ
)

// CauseGroup identifies which Cause CHOICE alternative was decoded.
type CauseGroup int

const (
	CauseGroupRadioNetwork CauseGroup = iota
	CauseGroupProtocol
	CauseGroupMisc
	CauseGroupChoiceExtension
)

// Cause is a decoded NRPPa Cause value.
type Cause struct {
	Group CauseGroup
	Value int64 // ENUMERATED ordinal within the group (n/a for choice-Extension)
}

// APPosition is a decoded NG-RANAccessPointPosition (TS 38.455 §9.2.2 / TS
// 23.032 ellipsoid point with uncertainty ellipse). LatitudeDegrees /
// LongitudeDegrees are the WGS-84 decimal-degree conversions.
type APPosition struct {
	LatitudeSign           int   // 0 = north, 1 = south
	Latitude               int64 // encoded magnitude (0..2^23-1)
	Longitude              int64 // encoded value (-2^23..2^23-1)
	DirectionOfAltitude    int   // 0 = height, 1 = depth
	Altitude               int64
	UncertaintySemiMajor   int64
	UncertaintySemiMinor   int64
	OrientationOfMajorAxis int64
	UncertaintyAltitude    int64
	Confidence             int64

	LatitudeDegrees  float64
	LongitudeDegrees float64
}

// ServingCell is the decoded NG-RAN-CGI (serving cell global identity).
type ServingCell struct {
	PLMNIdentity   []byte  // 3 octets
	NRCellIdentity *uint64 // 36-bit, present for NR cells
	EUTRACellID    *uint64 // 28-bit, present for E-UTRA cells
}

// ECIDResult is the gNB-supplied E-CID measurement result carried in an
// E-CIDMeasurementInitiationResponse.
type ECIDResult struct {
	ServingCell        ServingCell
	ServingCellTAC     []byte // 3 octets
	APPosition         *APPosition
	TimingAdvanceType1 *int64 // valueTimingAdvanceType1-EUTRA (0..7690)
	TimingAdvanceType2 *int64 // valueTimingAdvanceType2-EUTRA (0..7690)
}

// ECIDRequest is a decoded E-CIDMeasurementInitiationRequest.
type ECIDRequest struct {
	LMFUEMeasurementID    int64
	ReportCharacteristics int // 0 = onDemand, 1 = periodic
	MeasurementQuantities []MeasurementQuantityValue
}

// ECIDResponse is a decoded E-CIDMeasurementInitiationResponse.
type ECIDResponse struct {
	LMFUEMeasurementID int64
	RANUEMeasurementID int64
	Result             *ECIDResult
	CellPortionID      *int64
}

// ECIDFailure is a decoded E-CIDMeasurementInitiationFailure.
type ECIDFailure struct {
	LMFUEMeasurementID int64
	Cause              Cause
}

// ParsedPDU is the discriminated result of ParsePDU.
type ParsedPDU struct {
	Kind     MessageKind
	Request  *ECIDRequest
	Response *ECIDResponse
	Failure  *ECIDFailure
}

// reject and ignore return the NRPPa criticality wrappers used when building IEs.
func reject() nrppatype.Criticality {
	return nrppatype.Criticality{Value: nrppatype.CriticalityPresentReject}
}

func ignore() nrppatype.Criticality {
	return nrppatype.Criticality{Value: nrppatype.CriticalityPresentIgnore}
}
