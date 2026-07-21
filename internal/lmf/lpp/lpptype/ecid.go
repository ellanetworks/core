// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/internal/per"

// =====================================================================
// ECID-ProvideLocationInformation (TS 37.355 §6.5.3.1)
// =====================================================================

//	ECID-ProvideLocationInformation ::= SEQUENCE {
//	    ecid-SignalMeasurementInformation	ECID-SignalMeasurementInformation	OPTIONAL,
//	    ecid-Error						ECID-Error						OPTIONAL,
//	    ...
//	}
type ECIDProvideLocationInformation struct {
	_                                [0]struct{}                       `per:"extseq"`
	EcidSignalMeasurementInformation *ECIDSignalMeasurementInformation `per:",optional"`
	EcidError                        *ECIDError                        `per:",optional"`
}

// =====================================================================
// ECID-SignalMeasurementInformation (TS 37.355 §6.5.3.2)
// =====================================================================

//	ECID-SignalMeasurementInformation ::= SEQUENCE {
//	    primaryCellMeasuredResults	MeasuredResultsElement			OPTIONAL,
//	    measuredResultsList			MeasuredResultsList,
//	    ...
//	}
//
// MeasuredResultsList ::= SEQUENCE (SIZE(1..32)) OF MeasuredResultsElement
type ECIDSignalMeasurementInformation struct {
	_                          [0]struct{}              `per:"extseq"`
	PrimaryCellMeasuredResults *MeasuredResultsElement  `per:",optional"`
	MeasuredResultsList        []MeasuredResultsElement `per:"SEQUENCE-OF,size:1..32"`
	ArfcnEUTRAV9a0             *ARFCNValueEUTRAV9a0     `per:",optional"`
	NrsrpResultR14             *int64                   `per:",optional,range:0..113"`
	NrsrqResultR14             *int64                   `per:",optional,range:0..74"`
	CarrierFreqOffsetNBR14     *CarrierFreqOffsetNBR14  `per:",optional"`
	HyperSFNR14                *[]bool                  `per:",optional,size:10"`
	RsrpResultV1470            *int64                   `per:",optional,range:-17..-1"`
	RsrqResultV1470            *int64                   `per:",optional,range:-30..46"`
}

// =====================================================================
// MeasuredResultsElement (TS 37.355 §6.5.3.2, inline)
// =====================================================================

//	MeasuredResultsElement ::= SEQUENCE {
//	    physCellId				INTEGER (0..503),
//	    cellGlobalId			CellGlobalIdEUTRA-And-UTRA	OPTIONAL,
//	    arfcnEUTRA				ARFCN-ValueEUTRA,
//	    systemFrameNumber		BIT STRING (SIZE (10))		OPTIONAL,
//	    rsrp-Result				INTEGER (0..97)			OPTIONAL,
//	    rsrq-Result				INTEGER (0..34)			OPTIONAL,
//	    ue-RxTxTimeDiff			INTEGER (0..495)		OPTIONAL,
//	    ...,
//	    [[ arfcnEUTRA-v9a0		ARFCN-ValueEUTRA-v9a0		OPTIONAL ]],
//	    [[ nrsrp-Result-r14		INTEGER (0..113)			OPTIONAL,
//	       nrsrq-Result-r14		INTEGER (0..74)			OPTIONAL,
//	       carrierFreqOffsetNB-r14	CarrierFreqOffsetNB-r14	OPTIONAL,
//	       hyperSFN-r14			BIT STRING (SIZE (10))	OPTIONAL ]],
//	    [[ rsrp-Result-v1470	INTEGER (-17..-1)		OPTIONAL,
//	       rsrq-Result-v1470	INTEGER (-30..46)		OPTIONAL ]]]
type MeasuredResultsElement struct {
	_                      [0]struct{}               `per:"extseq"`
	PhysCellId             int64                     `per:",range:0..503"`
	CellGlobalId           *CellGlobalIdEUTRAAndUTRA `per:",optional"`
	ArfcnEUTRA             *ARFCNValueEUTRA          `per:",optional"`
	SystemFrameNumber      *[]bool                   `per:",optional,size:10"`
	RsrpResult             *int64                    `per:",optional,range:0..97"`
	RsrqResult             *int64                    `per:",optional,range:0..34"`
	UeRxTxTimeDiff         *int64                    `per:",optional,range:0..495"`
	ArfcnEUTRAV9a0         *ARFCNValueEUTRAV9a0      `per:",optional"`
	NrsrpResultR14         *int64                    `per:",optional,range:0..113"`
	NrsrqResultR14         *int64                    `per:",optional,range:0..74"`
	CarrierFreqOffsetNBR14 *CarrierFreqOffsetNBR14   `per:",optional"`
	HyperSFNR14            *[]bool                   `per:",optional,size:10"`
	RsrpResultV1470        *int64                    `per:",optional,range:-17..-1"`
	RsrqResultV1470        *int64                    `per:",optional,range:-30..46"`
}

// =====================================================================
// ECID-Error (TS 37.355 §6.5.3.6)
// =====================================================================

//	ECID-Error ::= CHOICE {
//	    locationServerErrorCauses	ECID-LocationServerErrorCauses,
//	    targetDeviceErrorCauses	ECID-TargetDeviceErrorCauses,
//	    ...
//	}
type ECIDError struct {
	_                         [0]struct{}                    `per:"extseq"`
	LocationServerErrorCauses *ECIDLocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *ECIDTargetDeviceErrorCauses   `per:",choice:1,optional"`
}

// =====================================================================
// ECID-LocationServerErrorCauses (TS 37.355 §6.5.3.6)
// =====================================================================

//	ECID-LocationServerErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, ... },
//	    ...
//	}
const (
	ECIDLocationServerErrorCausesUndefined int64 = 0
)

type ECIDLocationServerErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:",range:0..0,..."`
}

// =====================================================================
// ECID-TargetDeviceErrorCauses (TS 37.355 §6.5.3.6)
// =====================================================================

//	ECID-TargetDeviceErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, requestedMeasurementNotAvailable,
//	                       notAllrequestedMeasurementsPossible, ... },
//	    rsrpMeasurementNotPossible	NULL			OPTIONAL,
//	    rsrqMeasurementNotPossible	NULL			OPTIONAL,
//	    ueRxTxTimeDiffNotPossible	NULL			OPTIONAL,
//	    ...
//	}
const (
	ECIDTargetDeviceErrorCausesUndefined                           int64 = 0
	ECIDTargetDeviceErrorCausesRequestedMeasurementNotAvailable    int64 = 1
	ECIDTargetDeviceErrorCausesNotAllRequestedMeasurementsPossible int64 = 2
)

type ECIDTargetDeviceErrorCauses struct {
	_                          [0]struct{} `per:"extseq"`
	Cause                      int64       `per:",range:0..2,..."`
	RsrpMeasurementNotPossible *per.Null   `per:",optional"`
	RsrqMeasurementNotPossible *per.Null   `per:",optional"`
	UeRxTxTimeDiffNotPossible  *per.Null   `per:",optional"`
}

// =====================================================================
// CellGlobalIdEUTRA-And-UTRA (TS 37.355 §6.5.3)
// =====================================================================

type CellGlobalIdEUTRAAndUTRA struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	TACAndECID   ECGITACAndECID
}
