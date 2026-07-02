// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppatype

// =====================================================================
// E-CIDMeasurementInitiationRequest (NRPPA-PDU-Contents)
// =====================================================================

// E-CIDMeasurementInitiationRequest ::= SEQUENCE { protocolIEs ..., ... }.
type ECIDMeasurementInitiationRequest struct {
	ProtocolIEs ProtocolIEContainerECIDMeasurementInitiationRequestIEs
}

// ProtocolIE-Container ::= SEQUENCE (SIZE (0..maxProtocolIEs=65535)) OF
// ProtocolIE-Field.
type ProtocolIEContainerECIDMeasurementInitiationRequestIEs struct {
	List []ECIDMeasurementInitiationRequestIEs `aper:"sizeLB:0,sizeUB:65535"`
}

type ECIDMeasurementInitiationRequestIEs struct {
	Id          ProtocolIEID
	Criticality Criticality
	Value       ECIDMeasurementInitiationRequestIEsValue `aper:"openType,referenceFieldName:Id"`
}

const (
	ECIDMeasurementInitiationRequestIEsPresentNothing int = iota /* No components present */
	ECIDMeasurementInitiationRequestIEsPresentLMFUEMeasurementID
	ECIDMeasurementInitiationRequestIEsPresentReportCharacteristics
	ECIDMeasurementInitiationRequestIEsPresentMeasurementPeriodicity
	ECIDMeasurementInitiationRequestIEsPresentMeasurementQuantities
)

type ECIDMeasurementInitiationRequestIEsValue struct {
	Present                int
	LMFUEMeasurementID     *UEMeasurementID        `aper:"referenceFieldValue:2"`
	ReportCharacteristics  *ReportCharacteristics  `aper:"referenceFieldValue:3"`
	MeasurementPeriodicity *MeasurementPeriodicity `aper:"referenceFieldValue:4"`
	MeasurementQuantities  *MeasurementQuantities  `aper:"referenceFieldValue:5"`
}

// =====================================================================
// E-CIDMeasurementInitiationResponse (NRPPA-PDU-Contents)
// =====================================================================

// E-CIDMeasurementInitiationResponse ::= SEQUENCE { protocolIEs ..., ... }.
type ECIDMeasurementInitiationResponse struct {
	ProtocolIEs ProtocolIEContainerECIDMeasurementInitiationResponseIEs
}

type ProtocolIEContainerECIDMeasurementInitiationResponseIEs struct {
	List []ECIDMeasurementInitiationResponseIEs `aper:"sizeLB:0,sizeUB:65535"`
}

type ECIDMeasurementInitiationResponseIEs struct {
	Id          ProtocolIEID
	Criticality Criticality
	Value       ECIDMeasurementInitiationResponseIEsValue `aper:"openType,referenceFieldName:Id"`
}

const (
	ECIDMeasurementInitiationResponseIEsPresentNothing int = iota /* No components present */
	ECIDMeasurementInitiationResponseIEsPresentLMFUEMeasurementID
	ECIDMeasurementInitiationResponseIEsPresentRANUEMeasurementID
	ECIDMeasurementInitiationResponseIEsPresentECIDMeasurementResult
	ECIDMeasurementInitiationResponseIEsPresentCriticalityDiagnostics
	ECIDMeasurementInitiationResponseIEsPresentCellPortionID
)

type ECIDMeasurementInitiationResponseIEsValue struct {
	Present                int
	LMFUEMeasurementID     *UEMeasurementID        `aper:"referenceFieldValue:2"`
	RANUEMeasurementID     *UEMeasurementID        `aper:"referenceFieldValue:6"`
	ECIDMeasurementResult  *ECIDMeasurementResult  `aper:"valueExt,referenceFieldValue:7"`
	CriticalityDiagnostics *CriticalityDiagnostics `aper:"valueExt,referenceFieldValue:1"`
	CellPortionID          *CellPortionID          `aper:"referenceFieldValue:14"`
}

// =====================================================================
// E-CIDMeasurementInitiationFailure (NRPPA-PDU-Contents)
// =====================================================================

// E-CIDMeasurementInitiationFailure ::= SEQUENCE { protocolIEs ..., ... }.
type ECIDMeasurementInitiationFailure struct {
	ProtocolIEs ProtocolIEContainerECIDMeasurementInitiationFailureIEs
}

type ProtocolIEContainerECIDMeasurementInitiationFailureIEs struct {
	List []ECIDMeasurementInitiationFailureIEs `aper:"sizeLB:0,sizeUB:65535"`
}

type ECIDMeasurementInitiationFailureIEs struct {
	Id          ProtocolIEID
	Criticality Criticality
	Value       ECIDMeasurementInitiationFailureIEsValue `aper:"openType,referenceFieldName:Id"`
}

const (
	ECIDMeasurementInitiationFailureIEsPresentNothing int = iota /* No components present */
	ECIDMeasurementInitiationFailureIEsPresentLMFUEMeasurementID
	ECIDMeasurementInitiationFailureIEsPresentCause
	ECIDMeasurementInitiationFailureIEsPresentCriticalityDiagnostics
)

type ECIDMeasurementInitiationFailureIEsValue struct {
	Present                int
	LMFUEMeasurementID     *UEMeasurementID        `aper:"referenceFieldValue:2"`
	Cause                  *Cause                  `aper:"valueLB:0,valueUB:3,referenceFieldValue:0"`
	CriticalityDiagnostics *CriticalityDiagnostics `aper:"valueExt,referenceFieldValue:1"`
}

// =====================================================================
// E-CIDMeasurementTerminationCommand (NRPPA-PDU-Contents)
// =====================================================================

// E-CIDMeasurementTerminationCommand ::= SEQUENCE { protocolIEs ..., ... }.
// LMF → RAN, Class 2 elementary procedure (no response). Carries the
// LMF-UE-Measurement-ID and RAN-UE-Measurement-ID identifying the measurement
// association to release (TS 38.455 §9.1.4).
type ECIDMeasurementTerminationCommand struct {
	ProtocolIEs ProtocolIEContainerECIDMeasurementTerminationCommandIEs
}

type ProtocolIEContainerECIDMeasurementTerminationCommandIEs struct {
	List []ECIDMeasurementTerminationCommandIEs `aper:"sizeLB:0,sizeUB:65535"`
}

type ECIDMeasurementTerminationCommandIEs struct {
	Id          ProtocolIEID
	Criticality Criticality
	Value       ECIDMeasurementTerminationCommandIEsValue `aper:"openType,referenceFieldName:Id"`
}

const (
	ECIDMeasurementTerminationCommandIEsPresentNothing int = iota /* No components present */
	ECIDMeasurementTerminationCommandIEsPresentLMFUEMeasurementID
	ECIDMeasurementTerminationCommandIEsPresentRANUEMeasurementID
)

type ECIDMeasurementTerminationCommandIEsValue struct {
	Present            int
	LMFUEMeasurementID *UEMeasurementID `aper:"referenceFieldValue:2"`
	RANUEMeasurementID *UEMeasurementID `aper:"referenceFieldValue:6"`
}
