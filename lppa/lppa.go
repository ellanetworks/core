// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lppa implements the 3GPP TS 36.455 LPPa (LTE Positioning Protocol A)
// E-CID Measurement Initiation procedure in aligned PER.
//
// LPPa travels as an octet string inside S1AP UE-associated LPPa transport
// messages (TS 36.413 §8.14) between the E-SMLC/LMF and the eNB. Requests and
// responses are correlated by the E-SMLC-UE-Measurement-ID, echoed by the eNB.
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

// MeasurementQuantityValue holds the root values of TS 36.455
// MeasurementQuantitiesValue.
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

// APPosition is an E-UTRANAccessPointPosition (TS 36.455 §9.2.1), a TS 23.032
// ellipsoid point with altitude and uncertainty ellipse.
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

	// WGS-84 decimal degrees, derived on decode from the encoded values above.
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

// ECIDResult is an E-CID-MeasurementResult (TS 36.455 §9.2.5), with the
// MeasuredResults CHOICE list flattened onto the measurement fields below.
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
