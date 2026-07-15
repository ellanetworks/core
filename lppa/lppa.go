// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lppa implements a 3GPP TS 36.455 LPPa (LTE Positioning Protocol A)
// ASN.1 codec for the E-CID Measurement Initiation procedure, over the
// aligned-PER library github.com/ellanetworks/core/aper.
//
// LPPa is transported as an octet string inside S1AP UE-associated LPPa
// transport messages (TS 36.413 §8.14) between the E-SMLC/LMF and the eNB. This
// package builds the E-SMLC-originated messages (E-CID Measurement Initiation
// Request, Termination Command) and parses the eNB-originated ones (Response,
// Initiation Failure, Failure Indication) without exposing raw aper types.
//
// Request/response correlation is by the E-SMLC-UE-Measurement-ID, echoed by the
// eNB. The LPPa transaction id is carried in the elementary-procedure envelope.
package lppa

type MessageKind int

const (
	KindUnknown MessageKind = iota
	KindECIDMeasurementInitiationRequest
	KindECIDMeasurementInitiationResponse
	KindECIDMeasurementInitiationFailure
	KindECIDMeasurementTerminationCommand
	KindECIDMeasurementFailureIndication
)

// MeasurementQuantityValue enumerates the E-CID measurement quantities
// (TS 36.455 MeasurementQuantitiesValue, root values only).
type MeasurementQuantityValue int

const (
	MeasCellID MeasurementQuantityValue = iota
	MeasAngleOfArrival
	MeasTimingAdvanceType1
	MeasTimingAdvanceType2
	MeasRSRP
	MeasRSRQ
)

type CauseGroup int

const (
	CauseGroupRadioNetwork CauseGroup = iota
	CauseGroupProtocol
	CauseGroupMisc
	CauseGroupChoiceExtension
)

type Cause struct {
	Group CauseGroup
	Value int64 // ENUMERATED ordinal within the group (n/a for choice-Extension)
}

// APPosition is a decoded E-UTRANAccessPointPosition (TS 36.455 §9.2.1 / TS
// 23.032 ellipsoid point with altitude and uncertainty ellipse). LatitudeDegrees
// and LongitudeDegrees are the WGS-84 decimal-degree conversions.
type APPosition struct {
	LatitudeSign           int   // 0 = north, 1 = south
	Latitude               int64 // encoded magnitude (0..2^23-1)
	Longitude              int64 // encoded value (-2^23..2^23-1)
	DirectionOfAltitude    int   // 0 = height, 1 = depth
	Altitude               int64 // 0..32767
	UncertaintySemiMajor   int64 // 0..127
	UncertaintySemiMinor   int64 // 0..127
	OrientationOfMajorAxis int64 // 0..179
	UncertaintyAltitude    int64 // 0..127
	Confidence             int64 // 0..100

	LatitudeDegrees  float64
	LongitudeDegrees float64
}

type ECGI struct {
	PLMNIdentity []byte // 3 octets
	EUTRACellID  uint64 // 28-bit
}

type RSRPItem struct {
	PCI       int64 // 0..503
	EARFCN    int64 // 0..65535 (root)
	ECGI      *ECGI // optional
	ValueRSRP int64 // 0..97, TS 36.133 §9.1.4
}

type RSRQItem struct {
	PCI       int64
	EARFCN    int64
	ECGI      *ECGI
	ValueRSRQ int64 // 0..34, TS 36.133 §9.1.7
}

// ECIDResult is the eNB-supplied E-CID measurement result carried in an
// E-CIDMeasurementInitiationResponse (TS 36.455 §9.2.5). The MeasuredResults
// CHOICE list is flattened onto the measurement fields.
type ECIDResult struct {
	ServingCell    ECGI
	ServingCellTAC []byte // 2 octets

	APPosition *APPosition

	AngleOfArrival     *int64 // valueAngleOfArrival (0..719, degrees)
	TimingAdvanceType1 *int64 // valueTimingAdvanceType1 (0..7690)
	TimingAdvanceType2 *int64 // valueTimingAdvanceType2 (0..7690)
	RSRP               []RSRPItem
	RSRQ               []RSRQItem
}

type ECIDRequest struct {
	ESMLCUEMeasurementID  int64
	ReportCharacteristics int // 0 = onDemand, 1 = periodic
	MeasurementQuantities []MeasurementQuantityValue
}

type ECIDResponse struct {
	ESMLCUEMeasurementID int64
	ENBUEMeasurementID   int64
	Result               *ECIDResult
	CellPortionID        *int64
}

type ECIDFailure struct {
	ESMLCUEMeasurementID int64
	Cause                Cause
}

type ECIDFailureIndication struct {
	ESMLCUEMeasurementID int64
	ENBUEMeasurementID   int64
	Cause                Cause
}

type ECIDTermination struct {
	ESMLCUEMeasurementID int64
	ENBUEMeasurementID   int64
}

type ParsedPDU struct {
	Kind              MessageKind
	Request           *ECIDRequest
	Response          *ECIDResponse
	Failure           *ECIDFailure
	FailureIndication *ECIDFailureIndication
	Termination       *ECIDTermination
}
