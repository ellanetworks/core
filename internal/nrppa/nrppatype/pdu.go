// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppatype

//	NRPPaPDU ::= CHOICE {
//	    initiatingMessage   InitiatingMessage,
//	    successfulOutcome   SuccessfulOutcome,
//	    unsuccessfulOutcome UnsuccessfulOutcome,
//	    ...
//	}
//
// Extensible CHOICE of 3 root alternatives, encoded with the top-level params
// "valueExt,valueLB:0,valueUB:2" (identical to NGAP-PDU).
const (
	NRPPaPDUPresentNothing int = iota /* No components present */
	NRPPaPDUPresentInitiatingMessage
	NRPPaPDUPresentSuccessfulOutcome
	NRPPaPDUPresentUnsuccessfulOutcome
)

type NRPPaPDU struct {
	Present             int
	InitiatingMessage   *InitiatingMessage
	SuccessfulOutcome   *SuccessfulOutcome
	UnsuccessfulOutcome *UnsuccessfulOutcome
}

//	InitiatingMessage ::= SEQUENCE {
//	    procedureCode      ProcedureCode,
//	    criticality        Criticality,
//	    nrppatransactionID NRPPATransactionID,
//	    value              <open type by procedureCode>
//	}
//
// NOTE: unlike NGAP, the NRPPa elementary-procedure envelope carries a
// mandatory nrppatransactionID between criticality and value (TS 38.455 §9.2).
type InitiatingMessage struct {
	ProcedureCode      ProcedureCode
	Criticality        Criticality
	NRPPATransactionID NRPPATransactionID
	Value              InitiatingMessageValue `aper:"openType,referenceFieldName:ProcedureCode"`
}

const (
	InitiatingMessagePresentNothing int = iota /* No components present */
	InitiatingMessagePresentECIDMeasurementInitiationRequest
)

type InitiatingMessageValue struct {
	Present                          int
	ECIDMeasurementInitiationRequest *ECIDMeasurementInitiationRequest `aper:"valueExt,referenceFieldValue:2"`
}

//	SuccessfulOutcome ::= SEQUENCE {
//	    procedureCode, criticality, nrppatransactionID, value
//	}
type SuccessfulOutcome struct {
	ProcedureCode      ProcedureCode
	Criticality        Criticality
	NRPPATransactionID NRPPATransactionID
	Value              SuccessfulOutcomeValue `aper:"openType,referenceFieldName:ProcedureCode"`
}

const (
	SuccessfulOutcomePresentNothing int = iota /* No components present */
	SuccessfulOutcomePresentECIDMeasurementInitiationResponse
)

type SuccessfulOutcomeValue struct {
	Present                           int
	ECIDMeasurementInitiationResponse *ECIDMeasurementInitiationResponse `aper:"valueExt,referenceFieldValue:2"`
}

//	UnsuccessfulOutcome ::= SEQUENCE {
//	    procedureCode, criticality, nrppatransactionID, value
//	}
type UnsuccessfulOutcome struct {
	ProcedureCode      ProcedureCode
	Criticality        Criticality
	NRPPATransactionID NRPPATransactionID
	Value              UnsuccessfulOutcomeValue `aper:"openType,referenceFieldName:ProcedureCode"`
}

const (
	UnsuccessfulOutcomePresentNothing int = iota /* No components present */
	UnsuccessfulOutcomePresentECIDMeasurementInitiationFailure
)

type UnsuccessfulOutcomeValue struct {
	Present                          int
	ECIDMeasurementInitiationFailure *ECIDMeasurementInitiationFailure `aper:"valueExt,referenceFieldValue:2"`
}
