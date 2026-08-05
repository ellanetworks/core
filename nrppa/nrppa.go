// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nrppa implements the 3GPP TS 38.455 NRPPa (NR Positioning Protocol A)
// E-CID Measurement Initiation procedure in aligned PER.
//
// NRPPa travels as an octet string inside NGAP UE-associated NRPPa transport
// messages (TS 38.413 §8.14) between the LMF and the NG-RAN node. Requests and
// responses are correlated by the LMF-UE-Measurement-ID, echoed by the NG-RAN
// node.
package nrppa

type MessageKind int

const (
	KindUnknown MessageKind = iota
	KindECIDMeasurementInitiationRequest
	KindECIDMeasurementInitiationResponse
	KindECIDMeasurementInitiationFailure
	KindECIDMeasurementTerminationCommand
	KindECIDMeasurementFailureIndication
)

// MeasurementQuantityValue holds the values of TS 38.455
// MeasurementQuantitiesValue. The first six are the root; the NR quantities
// that follow are extension additions, which TS 36.455 has no counterpart for.
type MeasurementQuantityValue int

const (
	MeasCellID MeasurementQuantityValue = iota
	MeasAngleOfArrival
	MeasTimingAdvanceType1
	MeasTimingAdvanceType2
	MeasRSRP
	MeasRSRQ

	MeasSSRSRP
	MeasSSRSRQ
	MeasCSIRSRP
	MeasCSIRSRQ
	MeasAngleOfArrivalNR
	MeasTimingAdvanceNR
	MeasUERxTxTimeDiff
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

// APPosition is an NG-RANAccessPointPosition (TS 38.455 §9.2.2), a TS 23.032
// ellipsoid point with altitude and uncertainty ellipse. Field for field it is
// the E-UTRANAccessPointPosition of TS 36.455 §9.2.1.
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

// NGRANCGI is an NG-RAN-CGI (TS 38.455 §9.2.10). Unlike the E-UTRAN-only ECGI
// of TS 36.455 §9.2.9, the cell identity is a CHOICE: exactly one of
// NRCellIdentity (36 bits) or EUTRACellID (28 bits) is set.
type NGRANCGI struct {
	PLMNIdentity   []byte // 3 octets
	NRCellIdentity *uint64
	EUTRACellID    *uint64
}

// CGINR is a CGI-NR (TS 38.455 §9.2.12), the cell identity an NR per-cell
// measurement item may name. It has no TS 36.455 counterpart.
type CGINR struct {
	PLMNIdentity   []byte // 3 octets
	NRCellIdentity uint64 // 36-bit
}

// CGIEUTRA is a CGI-EUTRA (TS 38.455 §9.2.11), the cell identity an E-UTRA
// per-cell measurement item may name. It is the ECGI of TS 36.455 §9.2.9.
type CGIEUTRA struct {
	PLMNIdentity []byte // 3 octets
	EUTRACellID  uint64 // 28-bit
}

type RSRPItem struct {
	PCI       int64 // 0..503
	EARFCN    int64 // 0..262143 (root)
	CGI       *CGIEUTRA
	ValueRSRP int64 // 0..97, TS 36.133 §9.1.4
}

type RSRQItem struct {
	PCI       int64
	EARFCN    int64
	CGI       *CGIEUTRA
	ValueRSRQ int64 // 0..34, TS 36.133 §9.1.7
}

// SSRSRPItem is a ResultSS-RSRP-Item (TS 38.455 §9.2.32). Both the per-cell
// value and the per-SSB list are optional, and a report may carry either or
// both.
type SSRSRPItem struct {
	NRPCI   int64 // 0..1007
	NRARFCN int64 // 0..3279165
	CGI     *CGINR
	Value   *int64 // valueSS-RSRP-Cell, 0..127, TS 38.133 §10.1.6
	PerSSB  []SSBResultItem
}

type SSRSRQItem struct {
	NRPCI   int64
	NRARFCN int64
	CGI     *CGINR
	Value   *int64 // valueSS-RSRQ-Cell, 0..127, TS 38.133 §10.1.11
	PerSSB  []SSBResultItem
}

// SSBResultItem is one SSB-indexed measurement of a ResultSS-RSRP/RSRQ item.
type SSBResultItem struct {
	SSBIndex int64 // 0..63
	Value    int64 // 0..127
}

// CSIRSRPItem is a ResultCSI-RSRP-Item (TS 38.455 §9.2.34).
type CSIRSRPItem struct {
	NRPCI    int64
	NRARFCN  int64
	CGI      *CGINR
	Value    *int64 // valueCSI-RSRP-Cell, 0..127
	PerCSIRS []CSIRSResultItem
}

type CSIRSRQItem struct {
	NRPCI    int64
	NRARFCN  int64
	CGI      *CGINR
	Value    *int64 // valueCSI-RSRQ-Cell, 0..127
	PerCSIRS []CSIRSResultItem
}

// CSIRSResultItem is one CSI-RS-indexed measurement of a ResultCSI-RSRP/RSRQ
// item.
type CSIRSResultItem struct {
	CSIRSIndex int64 // 0..95
	Value      int64 // 0..127
}

// AoAResult is a UL-AoA (TS 38.455 §9.2.38). Angles are exposed both raw, in
// the 0.1-degree units the wire carries, and as decimal degrees. It has no
// TS 36.455 counterpart: LPPa reports only the E-UTRA valueAngleOfArrival.
type AoAResult struct {
	AzimuthRaw     int64   // 0..3599 (0.1° units)
	AzimuthDegrees float64 // 0..359.9
	ZenithRaw      *int64  // 0..1799 (0.1° units), optional
	ZenithDegrees  *float64
	LCSToGCS       *LCSToGCS // optional LCS→GCS rotation angles
}

// LCSToGCS is an LCS-to-GCS-Translation (TS 38.455 §9.2.70), converted from the
// 0.1-degree units on the wire to decimal degrees.
type LCSToGCS struct {
	AlphaDegrees float64
	BetaDegrees  float64
	GammaDegrees float64
}

// ECIDResult is an E-CID-MeasurementResult (TS 38.455 §9.2.3), with the
// MeasuredResults CHOICE list flattened onto the measurement fields below. The
// E-UTRA fields are shared with TS 36.455 §9.2.5; everything from SSRSRP down
// arrives as a choice-Extension and has no LPPa counterpart.
type ECIDResult struct {
	ServingCell    NGRANCGI
	ServingCellTAC []byte // 3 octets (TS 36.455 has 2)

	APPosition *APPosition

	AngleOfArrival     *int64 // valueAngleOfArrival-EUTRA (0..719, degrees)
	TimingAdvanceType1 *int64 // valueTimingAdvanceType1-EUTRA (0..7690)
	TimingAdvanceType2 *int64 // valueTimingAdvanceType2-EUTRA (0..7690)
	RSRP               []RSRPItem
	RSRQ               []RSRQItem

	SSRSRP  []SSRSRPItem
	SSRSRQ  []SSRSRQItem
	CSIRSRP []CSIRSRPItem
	CSIRSRQ []CSIRSRQItem

	AoA             *AoAResult // angleOfArrivalNR
	NRTimingAdvance *int64     // nR-TADV (0..7690), TS 38.133
	UERxTxTimeDiff  *int64     // uE-Rx-Tx-Time-Diff (0..61565), TS 38.215
}

type ECIDRequest struct {
	LMFUEMeasurementID    int64
	ReportCharacteristics int // 0 = onDemand, 1 = periodic
	MeasurementQuantities []MeasurementQuantityValue
}

type ECIDResponse struct {
	LMFUEMeasurementID int64
	RANUEMeasurementID int64
	Result             *ECIDResult
	CellPortionID      *int64
}

type ECIDFailure struct {
	LMFUEMeasurementID int64
	Cause              Cause
}

type ECIDFailureIndication struct {
	LMFUEMeasurementID int64
	RANUEMeasurementID int64
	Cause              Cause
}

type ECIDTermination struct {
	LMFUEMeasurementID int64
	RANUEMeasurementID int64
}

type ParsedPDU struct {
	Kind              MessageKind
	Request           *ECIDRequest
	Response          *ECIDResponse
	Failure           *ECIDFailure
	FailureIndication *ECIDFailureIndication
	Termination       *ECIDTermination
}
