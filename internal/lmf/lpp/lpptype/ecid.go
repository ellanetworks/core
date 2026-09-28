// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/per"

// =====================================================================
// ECID-RequestLocationInformation (TS 37.355 §6.5.4)
// =====================================================================

//	ECID-RequestLocationInformation ::= SEQUENCE {
//	    requestedMeasurements		BIT STRING {	rsrpReq		(0),
//											rsrqReq		(1),
//											ueRxTxReq	(2),
//											nrsrpReq-r14	(3),
//											nrsrqReq-r14	(4)} (SIZE(1..8)),
//	    ...
//	}
const (
	ECIDRequestedMeasurementsRSRPReq   = 0
	ECIDRequestedMeasurementsRSRQReq   = 1
	ECIDRequestedMeasurementsUERxTxReq = 2
	ECIDRequestedMeasurementsNRSRPReq  = 3
	ECIDRequestedMeasurementsNRSRQReq  = 4
)

type ECIDRequestLocationInformation struct {
	_                     [0]struct{} `per:"extseq"`
	RequestedMeasurements []bool      `per:",size:1..8"`
}

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
	ECIDSignalMeasurementInformation *ECIDSignalMeasurementInformation `per:",optional"`
	ECIDError                        *ECIDError                        `per:",optional"`
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
	_                          [0]struct{}             `per:"extseq"`
	PrimaryCellMeasuredResults *MeasuredResultsElement `per:",optional"`
	MeasuredResultsList        MeasuredResultsList
}

type MeasuredResultsList struct {
	List []MeasuredResultsElement `per:"SEQUENCE-OF,size:1..32"`
}

// =====================================================================
// MeasuredResultsElement (TS 37.355 §6.5.3.2, inline)
// =====================================================================

//	MeasuredResultsElement ::= SEQUENCE {
//	    physCellId				INTEGER (0..503),
//	    cellGlobalId			CellGlobalIdEUTRA-AndUTRA	OPTIONAL,
//	    arfcnEUTRA				ARFCN-ValueEUTRA,
//	    systemFrameNumber		BIT STRING (SIZE (10))		OPTIONAL,
//	    rsrp-Result				INTEGER (0..97)			OPTIONAL,
//	    rsrq-Result				INTEGER (0..34)			OPTIONAL,
//	    ue-RxTxTimeDiff			INTEGER (0..4095)		OPTIONAL,
//	    ...,
//	    [[ arfcnEUTRA-v9a0		ARFCN-ValueEUTRA-v9a0		OPTIONAL ]],
//	    [[ nrsrp-Result-r14		INTEGER (0..113)			OPTIONAL,
//	       nrsrq-Result-r14		INTEGER (0..74)			OPTIONAL,
//	       carrierFreqOffsetNB-r14	CarrierFreqOffsetNB-r14	OPTIONAL,
//	       hyperSFN-r14			BIT STRING (SIZE (10))	OPTIONAL ]],
//	    [[ rsrp-Result-v1470	INTEGER (-17..-1)		OPTIONAL,
//	       rsrq-Result-v1470	INTEGER (-30..46)		OPTIONAL ]]
//	}
type MeasuredResultsElement struct {
	_                 [0]struct{}                           `per:"extseq"`
	PhysCellID        int64                                 `per:",range:0..503"`
	CellGlobalID      *CellGlobalIdEUTRAAndUTRA             `per:",optional"`
	ARFCNEUTRA        int64                                 `per:",range:0..65535"`
	SystemFrameNumber []bool                                `per:",optional,size:10"`
	RSRPResult        *int64                                `per:",optional,range:0..97"`
	RSRQResult        *int64                                `per:",optional,range:0..34"`
	UERxTxTimeDiff    *int64                                `per:",optional,range:0..4095"`
	V9a0Additions     *MeasuredResultsElementV9a0Additions  `per:",ext"`
	R14Additions      *UnmodelledExtension                  `per:",ext"`
	V1470Additions    *MeasuredResultsElementV1470Additions `per:",ext"`
}

type MeasuredResultsElementV9a0Additions struct {
	ARFCNEUTRA *int64 `per:",optional,range:65536..262143"`
}

type MeasuredResultsElementV1470Additions struct {
	RSRPResult *int64 `per:",optional,range:-17..-1"`
	RSRQResult *int64 `per:",optional,range:-30..46"`
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
type ECIDLocationServerErrorCause int64

const (
	ECIDLocationServerErrorCauseUndefined ECIDLocationServerErrorCause = 0
)

type ECIDLocationServerErrorCauses struct {
	_     [0]struct{}                  `per:"extseq"`
	Cause ECIDLocationServerErrorCause `per:"ENUMERATED,range:0..0,...,extvalues:0"`
}

// =====================================================================
// ECID-TargetDeviceErrorCauses (TS 37.355 §6.5.3.6)
// =====================================================================

//	ECID-TargetDeviceErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, requestedMeasurementNotAvailable,
//	                       notAllrequestedMeasurementsPossible, ... },
//	    rsrpMeasurementNotPossible	NULL			OPTIONAL,
//	    rsrqMeasurementNotPossible	NULL			OPTIONAL,
//	    ueRxTxMeasurementNotPossible	NULL			OPTIONAL,
//	    ...
//	}
type ECIDTargetDeviceErrorCause int64

const (
	ECIDTargetDeviceErrorCauseUndefined                           ECIDTargetDeviceErrorCause = 0
	ECIDTargetDeviceErrorCauseRequestedMeasurementNotAvailable    ECIDTargetDeviceErrorCause = 1
	ECIDTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible ECIDTargetDeviceErrorCause = 2
)

type ECIDTargetDeviceErrorCauses struct {
	_                            [0]struct{}                `per:"extseq"`
	Cause                        ECIDTargetDeviceErrorCause `per:"ENUMERATED,range:0..2,...,extvalues:0"`
	RSRPMeasurementNotPossible   *per.Null                  `per:",optional"`
	RSRQMeasurementNotPossible   *per.Null                  `per:",optional"`
	UERxTxMeasurementNotPossible *per.Null                  `per:",optional"`
}
