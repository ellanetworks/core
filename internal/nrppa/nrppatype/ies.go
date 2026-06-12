// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppatype

import "github.com/free5gc/aper"

// =====================================================================
// Simple IE types (NRPPA-IEs)
// =====================================================================

// UE-Measurement-ID ::= INTEGER (1..15, ..., 16..256). Extensible INTEGER:
// root range is 1..15, so valueLB:1,valueUB:15 with valueExt.
type UEMeasurementID struct {
	Value int64 `aper:"valueLB:1,valueUB:15,valueExt"`
}

// ReportCharacteristics ::= ENUMERATED { onDemand, periodic, ... }. Extensible.
const (
	ReportCharacteristicsPresentOnDemand aper.Enumerated = 0
	ReportCharacteristicsPresentPeriodic aper.Enumerated = 1
)

type ReportCharacteristics struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:1,valueExt"`
}

// MeasurementPeriodicity ::= ENUMERATED { ms120, ms240, ms480, ms640, ms1024,
// ms2048, ms5120, ms10240, min1, min6, min12, min30, min60, ... } (13 root
// values, extensible).
const (
	MeasurementPeriodicityPresentMs120   aper.Enumerated = 0
	MeasurementPeriodicityPresentMs240   aper.Enumerated = 1
	MeasurementPeriodicityPresentMs480   aper.Enumerated = 2
	MeasurementPeriodicityPresentMs640   aper.Enumerated = 3
	MeasurementPeriodicityPresentMs1024  aper.Enumerated = 4
	MeasurementPeriodicityPresentMs2048  aper.Enumerated = 5
	MeasurementPeriodicityPresentMs5120  aper.Enumerated = 6
	MeasurementPeriodicityPresentMs10240 aper.Enumerated = 7
	MeasurementPeriodicityPresentMin1    aper.Enumerated = 8
	MeasurementPeriodicityPresentMin6    aper.Enumerated = 9
	MeasurementPeriodicityPresentMin12   aper.Enumerated = 10
	MeasurementPeriodicityPresentMin30   aper.Enumerated = 11
	MeasurementPeriodicityPresentMin60   aper.Enumerated = 12
)

type MeasurementPeriodicity struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:12,valueExt"`
}

// Cell-Portion-ID ::= INTEGER (0..4095, ...). Extensible INTEGER.
type CellPortionID struct {
	Value int64 `aper:"valueLB:0,valueUB:4095,valueExt"`
}

// PLMN-Identity ::= OCTET STRING (SIZE(3)).
type PLMNIdentity struct {
	Value aper.OctetString `aper:"sizeLB:3,sizeUB:3"`
}

// TAC ::= OCTET STRING (SIZE(3)).
type TAC struct {
	Value aper.OctetString `aper:"sizeLB:3,sizeUB:3"`
}

// EUTRACellIdentifier ::= BIT STRING (SIZE(28)).
type EUTRACellIdentifier struct {
	Value aper.BitString `aper:"sizeLB:28,sizeUB:28"`
}

// NRCellIdentifier ::= BIT STRING (SIZE(36)).
type NRCellIdentity struct {
	Value aper.BitString `aper:"sizeLB:36,sizeUB:36"`
}

// PCI-EUTRA ::= INTEGER (0..503, ...). Extensible.
type PCIEUTRA struct {
	Value int64 `aper:"valueLB:0,valueUB:503,valueExt"`
}

// EARFCN ::= INTEGER (0..262143, ...). Extensible.
type EARFCN struct {
	Value int64 `aper:"valueLB:0,valueUB:262143,valueExt"`
}

// ValueRSRP-EUTRA ::= INTEGER (0..97, ...). Extensible. (Verified TS 38.455.)
type ValueRSRPEUTRA struct {
	Value int64 `aper:"valueLB:0,valueUB:97,valueExt"`
}

// ValueRSRQ-EUTRA ::= INTEGER (0..34, ...). Extensible. (Verified TS 38.455 —
// the spec uses 0..34, not the -30..46 sometimes quoted.)
type ValueRSRQEUTRA struct {
	Value int64 `aper:"valueLB:0,valueUB:34,valueExt"`
}

// =====================================================================
// Cause (NRPPA-IEs)
// =====================================================================

// Cause ::= CHOICE { radioNetwork, protocol, misc, choice-Extension }.
// 4 alternatives; referenced with valueLB:0,valueUB:3 (no valueExt — the CHOICE
// has no "..." marker, extensibility is via choice-Extension).
const (
	CausePresentNothing int = iota /* No components present */
	CausePresentRadioNetwork
	CausePresentProtocol
	CausePresentMisc
	CausePresentChoiceExtension
)

type Cause struct {
	Present         int
	RadioNetwork    *CauseRadioNetwork
	Protocol        *CauseProtocol
	Misc            *CauseMisc
	ChoiceExtension *ProtocolIESingleContainerCauseExtensionIE
}

// CauseRadioNetwork ::= ENUMERATED { unspecified, requested-item-not-supported,
// requested-item-temporarily-not-available, ... } (3 root, extensible).
const (
	CauseRadioNetworkPresentUnspecified                          aper.Enumerated = 0
	CauseRadioNetworkPresentRequestedItemNotSupported            aper.Enumerated = 1
	CauseRadioNetworkPresentRequestedItemTemporarilyNotAvailable aper.Enumerated = 2
)

type CauseRadioNetwork struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:2,valueExt"`
}

// CauseProtocol ::= ENUMERATED { transfer-syntax-error,
// abstract-syntax-error-reject, abstract-syntax-error-ignore-and-notify,
// message-not-compatible-with-receiver-state, semantic-error, unspecified,
// abstract-syntax-error-falsely-constructed-message, ... } (7 root, extensible).
const (
	CauseProtocolPresentTransferSyntaxError                      aper.Enumerated = 0
	CauseProtocolPresentAbstractSyntaxErrorReject                aper.Enumerated = 1
	CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify       aper.Enumerated = 2
	CauseProtocolPresentMessageNotCompatibleWithReceiverState    aper.Enumerated = 3
	CauseProtocolPresentSemanticError                            aper.Enumerated = 4
	CauseProtocolPresentUnspecified                              aper.Enumerated = 5
	CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMsg aper.Enumerated = 6
)

type CauseProtocol struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:6,valueExt"`
}

// CauseMisc ::= ENUMERATED { unspecified, ... } (1 root, extensible).
const (
	CauseMiscPresentUnspecified aper.Enumerated = 0
)

type CauseMisc struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:0,valueExt"`
}

// =====================================================================
// CriticalityDiagnostics (NRPPA-IEs)
// =====================================================================

// CriticalityDiagnostics ::= SEQUENCE { procedureCode OPTIONAL,
// triggeringMessage OPTIONAL, procedureCriticality OPTIONAL, nrppatransactionID
// OPTIONAL, iEsCriticalityDiagnostics OPTIONAL, iE-Extensions OPTIONAL, ... }.
// Extensible — referenced with valueExt. The LMF only ever receives this IE
// (optional, usually absent); it is modelled fully so it decodes correctly.
type CriticalityDiagnostics struct {
	ProcedureCode             *ProcedureCode                                          `aper:"optional"`
	TriggeringMessage         *TriggeringMessage                                      `aper:"optional"`
	ProcedureCriticality      *Criticality                                            `aper:"optional"`
	NRPPATransactionID        *NRPPATransactionID                                     `aper:"optional"`
	IEsCriticalityDiagnostics *CriticalityDiagnosticsIEList                           `aper:"optional"`
	IEExtensions              *ProtocolExtensionContainerCriticalityDiagnosticsExtIEs `aper:"optional"`
}

// CriticalityDiagnostics-IE-List ::= SEQUENCE (SIZE (1..maxNrOfErrors=256)) OF
// the inner (extensible) item SEQUENCE.
type CriticalityDiagnosticsIEList struct {
	List []CriticalityDiagnosticsIEItem `aper:"valueExt,sizeLB:1,sizeUB:256"`
}

type CriticalityDiagnosticsIEItem struct {
	IECriticality Criticality
	IEID          ProtocolIEID
	TypeOfError   TypeOfError
	IEExtensions  *ProtocolExtensionContainerCriticalityDiagnosticsIEListExtIEs `aper:"optional"`
}

// =====================================================================
// MeasurementQuantities (NRPPA-IEs)
// =====================================================================

// MeasurementQuantities ::= SEQUENCE (SIZE (1..maxNoMeas=64)) OF
// ProtocolIE-Single-Container{{MeasurementQuantities-ItemIEs}}. Each element is
// the id/criticality/value ProtocolIE-Field triple. Size not extensible.
type MeasurementQuantities struct {
	List []MeasurementQuantitiesIEs `aper:"sizeLB:1,sizeUB:64"`
}

type MeasurementQuantitiesIEs struct {
	Id          ProtocolIEID
	Criticality Criticality
	Value       MeasurementQuantitiesIEsValue `aper:"openType,referenceFieldName:Id"`
}

const (
	MeasurementQuantitiesIEsPresentNothing int = iota /* No components present */
	MeasurementQuantitiesIEsPresentMeasurementQuantitiesItem
)

type MeasurementQuantitiesIEsValue struct {
	Present                   int
	MeasurementQuantitiesItem *MeasurementQuantitiesItem `aper:"valueExt,referenceFieldValue:11"`
}

// MeasurementQuantities-Item ::= SEQUENCE { measurementQuantitiesValue,
// iE-Extensions OPTIONAL, ... }. Extensible.
type MeasurementQuantitiesItem struct {
	MeasurementQuantitiesValue MeasurementQuantitiesValue
	IEExtensions               *ProtocolExtensionContainerMeasurementQuantitiesItemExtIEs `aper:"optional"`
}

// MeasurementQuantitiesValue ::= ENUMERATED { cell-ID, angleOfArrival,
// timingAdvanceType1, timingAdvanceType2, rSRP, rSRQ, ..., <NR variants> }.
// 6 root values, extensible.
const (
	MeasurementQuantitiesValuePresentCellID             aper.Enumerated = 0
	MeasurementQuantitiesValuePresentAngleOfArrival     aper.Enumerated = 1
	MeasurementQuantitiesValuePresentTimingAdvanceType1 aper.Enumerated = 2
	MeasurementQuantitiesValuePresentTimingAdvanceType2 aper.Enumerated = 3
	MeasurementQuantitiesValuePresentRSRP               aper.Enumerated = 4
	MeasurementQuantitiesValuePresentRSRQ               aper.Enumerated = 5
)

type MeasurementQuantitiesValue struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:5,valueExt"`
}

// =====================================================================
// E-CID-MeasurementResult graph (NRPPA-IEs)
// =====================================================================

//	E-CID-MeasurementResult ::= SEQUENCE {
//	    servingCell-ID            NG-RAN-CGI,
//	    servingCellTAC            TAC,
//	    nG-RANAccessPointPosition NG-RANAccessPointPosition OPTIONAL,
//	    measuredResults           MeasuredResults OPTIONAL,
//	    iE-Extensions             ... OPTIONAL,
//	    ...
//	}
//
// Extensible — referenced with valueExt.
type ECIDMeasurementResult struct {
	ServingCellID            NGRANCGI `aper:"valueExt"`
	ServingCellTAC           TAC
	NGRANAccessPointPosition *NGRANAccessPointPosition                              `aper:"optional,valueExt"`
	MeasuredResults          *MeasuredResults                                       `aper:"optional"`
	IEExtensions             *ProtocolExtensionContainerECIDMeasurementResultExtIEs `aper:"optional"`
}

// NG-RAN-CGI ::= SEQUENCE { pLMN-Identity, nG-RANcell NG-RANCell,
// iE-Extensions OPTIONAL, ... }. Extensible. The nG-RANcell field references an
// (non-"..."-extensible) CHOICE of 3 alternatives → valueLB:0,valueUB:2.
type NGRANCGI struct {
	PLMNIdentity PLMNIdentity
	NGRANCell    NGRANCell                                 `aper:"valueLB:0,valueUB:2"`
	IEExtensions *ProtocolExtensionContainerNGRANCGIExtIEs `aper:"optional"`
}

// NG-RANCell ::= CHOICE { eUTRA-CellID EUTRACellIdentifier,
// nR-CellID NRCellIdentifier, choice-Extension ... }. 3 alternatives.
const (
	NGRANCellPresentNothing int = iota /* No components present */
	NGRANCellPresentEUTRACellID
	NGRANCellPresentNRCellID
	NGRANCellPresentChoiceExtension
)

type NGRANCell struct {
	Present         int
	EUTRACellID     *EUTRACellIdentifier
	NRCellID        *NRCellIdentity
	ChoiceExtension *ProtocolIESingleContainerNGRANCellExtensionIE
}

//	NG-RANAccessPointPosition ::= SEQUENCE {
//	    latitudeSign ENUMERATED {north, south},
//	    latitude INTEGER (0..8388607),
//	    longitude INTEGER (-8388608..8388607),
//	    directionOfAltitude ENUMERATED {height, depth},
//	    altitude INTEGER (0..32767),
//	    uncertaintySemi-major INTEGER (0..127),
//	    uncertaintySemi-minor INTEGER (0..127),
//	    orientationOfMajorAxis INTEGER (0..179),
//	    uncertaintyAltitude INTEGER (0..127),
//	    confidence INTEGER (0..100),
//	    iE-Extensions ProtocolExtensionContainer {{ NG-RANAccessPointPosition-ExtIEs }} OPTIONAL,
//	    ...
//	}
//
// Per TS 38.455 (Rel-18) this SEQUENCE is extensible and carries an optional
// iE-Extensions; the field referencing it therefore uses "valueExt". The
// ENUMERATED sub-fields are inline aper.Enumerated, the INTEGER sub-fields int64.
type NGRANAccessPointPosition struct {
	LatitudeSign           aper.Enumerated                                           `aper:"valueLB:0,valueUB:1"`
	Latitude               int64                                                     `aper:"valueLB:0,valueUB:8388607"`
	Longitude              int64                                                     `aper:"valueLB:-8388608,valueUB:8388607"`
	DirectionOfAltitude    aper.Enumerated                                           `aper:"valueLB:0,valueUB:1"`
	Altitude               int64                                                     `aper:"valueLB:0,valueUB:32767"`
	UncertaintySemiMajor   int64                                                     `aper:"valueLB:0,valueUB:127"`
	UncertaintySemiMinor   int64                                                     `aper:"valueLB:0,valueUB:127"`
	OrientationOfMajorAxis int64                                                     `aper:"valueLB:0,valueUB:179"`
	UncertaintyAltitude    int64                                                     `aper:"valueLB:0,valueUB:127"`
	Confidence             int64                                                     `aper:"valueLB:0,valueUB:100"`
	IEExtensions           *ProtocolExtensionContainerNGRANAccessPointPositionExtIEs `aper:"optional"`
}

// NG-RANAccessPointPosition latitudeSign / directionOfAltitude enum values.
const (
	NGRANAccessPointPositionLatitudeSignNorth aper.Enumerated = 0
	NGRANAccessPointPositionLatitudeSignSouth aper.Enumerated = 1

	NGRANAccessPointPositionDirectionOfAltitudeHeight aper.Enumerated = 0
	NGRANAccessPointPositionDirectionOfAltitudeDepth  aper.Enumerated = 1
)

// MeasuredResults ::= SEQUENCE (SIZE (1..maxNoMeas=64)) OF MeasuredResultsValue.
// Element is a (non-"..."-extensible) CHOICE of 6 alternatives → the List field
// carries both the size bounds and valueLB:0,valueUB:5 (propagated to elements).
type MeasuredResults struct {
	List []MeasuredResultsValue `aper:"sizeLB:1,sizeUB:64,valueLB:0,valueUB:5"`
}

//	MeasuredResultsValue ::= CHOICE {
//	    valueAngleOfArrival-EUTRA     INTEGER (0..719),
//	    valueTimingAdvanceType1-EUTRA INTEGER (0..7690),
//	    valueTimingAdvanceType2-EUTRA INTEGER (0..7690),
//	    resultRSRP-EUTRA              ResultRSRP-EUTRA,
//	    resultRSRQ-EUTRA              ResultRSRQ-EUTRA,
//	    choice-Extension              ...
//	}
const (
	MeasuredResultsValuePresentNothing int = iota /* No components present */
	MeasuredResultsValuePresentValueAngleOfArrivalEUTRA
	MeasuredResultsValuePresentValueTimingAdvanceType1EUTRA
	MeasuredResultsValuePresentValueTimingAdvanceType2EUTRA
	MeasuredResultsValuePresentResultRSRPEUTRA
	MeasuredResultsValuePresentResultRSRQEUTRA
	MeasuredResultsValuePresentChoiceExtension
)

type MeasuredResultsValue struct {
	Present                      int
	ValueAngleOfArrivalEUTRA     *int64 `aper:"valueLB:0,valueUB:719"`
	ValueTimingAdvanceType1EUTRA *int64 `aper:"valueLB:0,valueUB:7690"`
	ValueTimingAdvanceType2EUTRA *int64 `aper:"valueLB:0,valueUB:7690"`
	ResultRSRPEUTRA              *ResultRSRPEUTRA
	ResultRSRQEUTRA              *ResultRSRQEUTRA
	ChoiceExtension              *ProtocolIESingleContainerMeasuredResultsValueExtensionIE
}

// ResultRSRP-EUTRA ::= SEQUENCE (SIZE (1..maxCellReport=9)) OF
// ResultRSRP-EUTRA-Item. The item SEQUENCE is extensible → valueExt on List.
//
// NOTE: the Rel-18 item also carries an optional cGI-EUTRA field between eARFCN
// and valueRSRP-EUTRA; this MVP omits it (these EUTRA result lists are never
// produced by an NR gNB). Decoding a Rel-18 item that includes cGI-EUTRA would
// therefore be incorrect.
type ResultRSRPEUTRA struct {
	List []ResultRSRPEUTRAItem `aper:"valueExt,sizeLB:1,sizeUB:9"`
}

type ResultRSRPEUTRAItem struct {
	PCIEUTRA       PCIEUTRA
	EARFCN         EARFCN
	ValueRSRPEUTRA ValueRSRPEUTRA
	IEExtensions   *ProtocolExtensionContainerResultRSRPEUTRAItemExtIEs `aper:"optional"`
}

// ResultRSRQ-EUTRA ::= SEQUENCE (SIZE (1..maxCellReport=9)) OF
// ResultRSRQ-EUTRA-Item.
type ResultRSRQEUTRA struct {
	List []ResultRSRQEUTRAItem `aper:"valueExt,sizeLB:1,sizeUB:9"`
}

type ResultRSRQEUTRAItem struct {
	PCIEUTRA       PCIEUTRA
	EARFCN         EARFCN
	ValueRSRQEUTRA ValueRSRQEUTRA
	IEExtensions   *ProtocolExtensionContainerResultRSRQEUTRAItemExtIEs `aper:"optional"`
}
