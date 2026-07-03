// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nrppatype holds hand-written, aper-tagged Go structs that mirror the
// conventions of free5gc's ngapType package. It implements the subset of the
// 3GPP TS 38.455 NRPPa (NR Positioning Protocol A) ASN.1 needed for the E-CID
// Measurement Initiation procedure (procedureCode = 2).
//
// The aligned-PER codec is github.com/free5gc/aper. The tag rules are verified
// against aper/common.go and the generated ngapType package; see internal/nrppa
// for the encode/decode entry points and the high-level helper API.
package nrppatype

import "github.com/free5gc/aper"

// Criticality ::= ENUMERATED { reject, ignore, notify } (NRPPA-CommonDataTypes).
// Not extensible.
const (
	CriticalityPresentReject aper.Enumerated = 0
	CriticalityPresentIgnore aper.Enumerated = 1
	CriticalityPresentNotify aper.Enumerated = 2
)

type Criticality struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:2"`
}

// ProcedureCode ::= INTEGER (0..255) (NRPPA-CommonDataTypes).
type ProcedureCode struct {
	Value int64 `aper:"valueLB:0,valueUB:255"`
}

// NRPPa procedure codes (NRPPA-Constants).
const (
	ProcedureCodeErrorIndication                  int64 = 0
	ProcedureCodePrivateMessage                   int64 = 1
	ProcedureCodeECIDMeasurementInitiation        int64 = 2
	ProcedureCodeECIDMeasurementFailureIndication int64 = 3
	ProcedureCodeECIDMeasurementReport            int64 = 4
	ProcedureCodeECIDMeasurementTermination       int64 = 5
)

// ProtocolIE-ID ::= INTEGER (0..maxProtocolIEs) with maxProtocolIEs = 65535.
type ProtocolIEID struct {
	Value int64 `aper:"valueLB:0,valueUB:65535"`
}

// NRPPa Protocol IE IDs (NRPPA-Constants) needed for the E-CID subset.
const (
	ProtocolIEIDCause                     int64 = 0
	ProtocolIEIDCriticalityDiagnostics    int64 = 1
	ProtocolIEIDLMFUEMeasurementID        int64 = 2
	ProtocolIEIDReportCharacteristics     int64 = 3
	ProtocolIEIDMeasurementPeriodicity    int64 = 4
	ProtocolIEIDMeasurementQuantities     int64 = 5
	ProtocolIEIDRANUEMeasurementID        int64 = 6
	ProtocolIEIDECIDMeasurementResult     int64 = 7
	ProtocolIEIDMeasurementQuantitiesItem int64 = 11
	ProtocolIEIDCellPortionID             int64 = 14

	// Extension IE ProtocolIEIDs for MeasuredResultsValue-ExtensionIE (TS 38.455 Rel-18).
	ProtocolIEIDResultSSRSRP     int64 = 32
	ProtocolIEIDResultSSRSRQ     int64 = 33
	ProtocolIEIDResultCSIRSRP    int64 = 34
	ProtocolIEIDResultCSIRSRQ    int64 = 35
	ProtocolIEIDAngleOfArrivalNR int64 = 36
	ProtocolIEIDNRTADV           int64 = 94
	ProtocolIEIDUERxTxTimeDiff   int64 = 118
)

// NRPPATransactionID ::= INTEGER (0..32767) (NRPPA-CommonDataTypes).
type NRPPATransactionID struct {
	Value int64 `aper:"valueLB:0,valueUB:32767"`
}

// TriggeringMessage ::= ENUMERATED { initiating-message, successful-outcome,
// unsuccessful-outcome } (NRPPA-CommonDataTypes). Not extensible.
const (
	TriggeringMessagePresentInitiatingMessage   aper.Enumerated = 0
	TriggeringMessagePresentSuccessfulOutcome   aper.Enumerated = 1
	TriggeringMessagePresentUnsuccessfulOutcome aper.Enumerated = 2
)

type TriggeringMessage struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:2"`
}

// TypeOfError ::= ENUMERATED { not-understood, missing, ... } (NRPPA-IEs).
// Extensible (2 root values).
const (
	TypeOfErrorPresentNotUnderstood aper.Enumerated = 0
	TypeOfErrorPresentMissing       aper.Enumerated = 1
)

type TypeOfError struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:1,valueExt"`
}
