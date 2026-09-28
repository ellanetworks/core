// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

// =====================================================================
// OTDOA-RequestLocationInformation (TS 37.355 §6.5.3)
// =====================================================================

//	OTDOA-RequestLocationInformation ::= SEQUENCE {
//	    assistanceAvailability		BOOLEAN,
//	    ...,
//	    [[
//	        multipathRSTD-r14		ENUMERATED { requested }	OPTIONAL,
//	        maxNoOfRSTDmeas-r14		INTEGER (1..32)				OPTIONAL
//	    ]],
//	    [[
//	        motionMeasurements-r15	ENUMERATED { requested }	OPTIONAL
//	    ]]
//	}
type OTDOARequestLocationInformation struct {
	_                      [0]struct{} `per:"extseq"`
	AssistanceAvailability bool
}

// =====================================================================
// OTDOA-ProvideLocationInformation (TS 37.355 §6.5.1.4)
// =====================================================================

//	OTDOA-ProvideLocationInformation ::= SEQUENCE {
//	    otdoaSignalMeasurementInformation	OTDOA-SignalMeasurementInformation	OPTIONAL,
//	    otdoa-Error						OTDOA-Error							OPTIONAL,
//	    ...,
//	    [[
//	        otdoaSignalMeasurementInformation-NB-r14	OTDOA-SignalMeasurementInformation-NB-r14	OPTIONAL
//	    ]]
//	}
type OTDOAProvideLocationInformation struct {
	_                                 [0]struct{}                        `per:"extseq"`
	OTDOASignalMeasurementInformation *OTDOASignalMeasurementInformation `per:",optional"`
	OTDOAError                        *OTDOAError                        `per:",optional"`
}

// =====================================================================
// OTDOA-SignalMeasurementInformation (TS 37.355 §6.5.1.5)
// =====================================================================

//	OTDOA-SignalMeasurementInformation ::= SEQUENCE {
//	    systemFrameNumber		BIT STRING (SIZE (10)),
//	    physCellIdRef			INTEGER (0..503),
//	    cellGlobalIdRef			ECGI					OPTIONAL,
//	    earfcnRef				ARFCN-ValueEUTRA		OPTIONAL,
//	    referenceQuality		OTDOA-MeasQuality		OPTIONAL,
//	    neighbourMeasurementList	NeighbourMeasurementList,
//	    ...,
//	    [[
//	        earfcnRef-v9a0		ARFCN-ValueEUTRA-v9a0	OPTIONAL
//	    ]],
//	    [[
//	        tpIdRef-r14			INTEGER (0..4095)				OPTIONAL,
//	        prsIdRef-r14			INTEGER (0..4095)				OPTIONAL,
//	        additionalPathsRef-r14	AdditionalPathList-r14		OPTIONAL,
//	        nprsIdRef-r14			INTEGER (0..4095)			OPTIONAL,
//	        carrierFreqOffsetNB-Ref-r14	CarrierFreqOffsetNB-r14	OPTIONAL,
//	        hyperSFN-r14			BIT STRING (SIZE (10))	OPTIONAL
//	    ]],
//	    [[
//	        motionTimeSource-r15	MotionTimeSource-r15	OPTIONAL
//	    ]]
//	}
//
// NeighbourMeasurementList ::= SEQUENCE (SIZE(1..24)) OF NeighbourMeasurementElement
type OTDOASignalMeasurementInformation struct {
	_                        [0]struct{}       `per:"extseq"`
	SystemFrameNumber        []bool            `per:",size:10"`
	PhysCellIDRef            int64             `per:",range:0..503"`
	CellGlobalIDRef          *ECGI             `per:",optional"`
	EARFCNRef                *int64            `per:",optional,range:0..65535"`
	ReferenceQuality         *OTDOAMeasQuality `per:",optional"`
	NeighbourMeasurementList NeighbourMeasurementList
}

type OTDOAMeasQuality struct {
	_               [0]struct{} `per:"extseq"`
	ErrorResolution []bool      `per:",size:2"`
	ErrorValue      []bool      `per:",size:5"`
	ErrorNumSamples []bool      `per:",optional,size:3"`
}

type NeighbourMeasurementList struct {
	List []NeighbourMeasurementElement `per:"SEQUENCE-OF,size:1..24"`
}

//	NeighbourMeasurementElement ::= SEQUENCE {
//	    physCellIdNeighbour		INTEGER (0..503),
//	    cellGlobalIdNeighbour	ECGI					OPTIONAL,
//	    earfcnNeighbour			ARFCN-ValueEUTRA		OPTIONAL,
//	    rstd					INTEGER (0..12711),
//	    rstd-Quality			OTDOA-MeasQuality,
//	    ...,
//	    [[ earfcnNeighbour-v9a0	ARFCN-ValueEUTRA-v9a0	OPTIONAL ]],
//	    [[ ... ]],
//	    [[ delta-SFN-r15		INTEGER (-8192..8191)	OPTIONAL ]]
//	}
type NeighbourMeasurementElement struct {
	_                     [0]struct{} `per:"extseq"`
	PhysCellIDNeighbour   int64       `per:",range:0..503"`
	CellGlobalIDNeighbour *ECGI       `per:",optional"`
	EARFCNNeighbour       *int64      `per:",optional,range:0..65535"`
	RSTD                  int64       `per:",range:0..12711"`
	RSTDQuality           OTDOAMeasQuality
}

// =====================================================================
// OTDOA-Error (TS 37.355 §6.5.1.9)
// =====================================================================

//	OTDOA-Error ::= CHOICE {
//	    locationServerErrorCauses	OTDOA-LocationServerErrorCauses,
//	    targetDeviceErrorCauses	OTDOA-TargetDeviceErrorCauses,
//	    ...
//	}
type OTDOAError struct {
	_                         [0]struct{}                     `per:"extseq"`
	LocationServerErrorCauses *OTDOALocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *OTDOATargetDeviceErrorCauses   `per:",choice:1,optional"`
}

//	OTDOA-LocationServerErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, assistanceDataNotSupportedByServer,
//	                       assistanceDataSupportedButCurrentlyNotAvailableByServer, ... },
//	    ...
//	}
type OTDOALocationServerErrorCause int64

const (
	OTDOALocationServerErrorCauseUndefined                                               OTDOALocationServerErrorCause = 0
	OTDOALocationServerErrorCauseAssistanceDataNotSupportedByServer                      OTDOALocationServerErrorCause = 1
	OTDOALocationServerErrorCauseAssistanceDataSupportedButCurrentlyNotAvailableByServer OTDOALocationServerErrorCause = 2
)

type OTDOALocationServerErrorCauses struct {
	_     [0]struct{}                   `per:"extseq"`
	Cause OTDOALocationServerErrorCause `per:"ENUMERATED,range:0..2,...,extvalues:0"`
}

//	OTDOA-TargetDeviceErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, assistance-data-missing,
//	                       unableToMeasureReferenceCell, unableToMeasureAnyNeighbourCell,
//	                       attemptedButUnableToMeasureSomeNeighbourCells, ... },
//	    ...
//	}
type OTDOATargetDeviceErrorCause int64

const (
	OTDOATargetDeviceErrorCauseUndefined                                     OTDOATargetDeviceErrorCause = 0
	OTDOATargetDeviceErrorCauseAssistanceDataMissing                         OTDOATargetDeviceErrorCause = 1
	OTDOATargetDeviceErrorCauseUnableToMeasureReferenceCell                  OTDOATargetDeviceErrorCause = 2
	OTDOATargetDeviceErrorCauseUnableToMeasureAnyNeighbourCell               OTDOATargetDeviceErrorCause = 3
	OTDOATargetDeviceErrorCauseAttemptedButUnableToMeasureSomeNeighbourCells OTDOATargetDeviceErrorCause = 4
)

type OTDOATargetDeviceErrorCauses struct {
	_     [0]struct{}                 `per:"extseq"`
	Cause OTDOATargetDeviceErrorCause `per:"ENUMERATED,range:0..4,...,extvalues:0"`
}

// =====================================================================
// OTDOA-RequestAssistanceData (TS 37.355 §6.5.1.3)
// =====================================================================

//	OTDOA-RequestAssistanceData ::= SEQUENCE {
//	    physCellId				INTEGER (0..503),
//	    ...,
//	    [[
//	        adType-r14			BIT STRING { prs (0), nprs (1) }	(SIZE (1..8))	OPTIONAL
//	    ]],
//	    [[
//	        nrPhysCellId-r15		INTEGER (0..1007)		OPTIONAL
//	    ]]
//	}
type OTDOARequestAssistanceData struct {
	_          [0]struct{} `per:"extseq"`
	PhysCellID int64       `per:",range:0..503"`
}

// =====================================================================
// OTDOA-ProvideAssistanceData (TS 37.355 §6.5.1.1)
// =====================================================================

//	OTDOA-ProvideAssistanceData ::= SEQUENCE {
//	    otdoa-ReferenceCellInfo	OTDOA-ReferenceCellInfo	OPTIONAL,
//	    otdoa-NeighbourCellInfo	OTDOA-NeighbourCellInfoList	OPTIONAL,
//	    otdoa-Error				OTDOA-Error				OPTIONAL,
//	    ...,
//	    [[
//	        otdoa-ReferenceCellInfoNB-r14	OTDOA-ReferenceCellInfoNB-r14	OPTIONAL,
//	        otdoa-NeighbourCellInfoNB-r14	OTDOA-NeighbourCellInfoListNB-r14	OPTIONAL
//	    ]]
//	}
type OTDOAProvideAssistanceData struct {
	_                      [0]struct{}                 `per:"extseq"`
	OTDOAReferenceCellInfo *OTDOAReferenceCellInfo     `per:",optional"`
	OTDOANeighbourCellInfo *OTDOANeighbourCellInfoList `per:",optional"`
	OTDOAError             *OTDOAError                 `per:",optional"`
}

type AntennaPortConfig int64

const (
	AntennaPortConfigPorts1Or2 AntennaPortConfig = 0
	AntennaPortConfigPorts4    AntennaPortConfig = 1
)

type CPLength int64

const (
	CPLengthNormal   CPLength = 0
	CPLengthExtended CPLength = 1
)

// =====================================================================
// OTDOA-ReferenceCellInfo (TS 37.355 §6.5.1.2)
// =====================================================================

//	OTDOA-ReferenceCellInfo ::= SEQUENCE {
//	    physCellId				INTEGER (0..503),
//	    cellGlobalId			ECGI					OPTIONAL,
//	    earfcnRef				ARFCN-ValueEUTRA		OPTIONAL,
//	    antennaPortConfig		ENUMERATED {ports1-or-2, ports4, ...}	OPTIONAL,
//	    cpLength				ENUMERATED { normal, extended, ... },
//	    prsInfo					PRS-Info				OPTIONAL,
//	    ...,
//	    [[
//	        earfcnRef-v9a0		ARFCN-ValueEUTRA-v9a0	OPTIONAL
//	    ]],
//	    [[
//	        tpId-r14				INTEGER (0..4095)				OPTIONAL,
//	        cpLengthCRS-r14		ENUMERATED { normal, extended, ... }	OPTIONAL,
//	        sameMBSFNconfigRef-r14	BOOLEAN		OPTIONAL,
//	        dlBandwidth-r14		ENUMERATED {n6, n15, n25, n50, n75, n100}	OPTIONAL,
//	        addPRSconfigRef-r14	SEQUENCE (SIZE (1..maxAddPRSconfig-r14)) OF PRS-Info	OPTIONAL
//	    ]],
//	    [[
//	        nr-LTE-SFN-Offset-r15		INTEGER (0..1023)			OPTIONAL
//	    ]],
//	    [[
//	        tdd-config-v1520			TDD-Config-v1520			OPTIONAL,
//	        nr-LTE-fineTiming-Offset-r15	INTEGER (0..19)			OPTIONAL
//	    ]]
//	}
type OTDOAReferenceCellInfo struct {
	_                 [0]struct{}        `per:"extseq"`
	PhysCellID        int64              `per:",range:0..503"`
	CellGlobalID      *ECGI              `per:",optional"`
	EARFCNRef         *int64             `per:",optional,range:0..65535"`
	AntennaPortConfig *AntennaPortConfig `per:"ENUMERATED,optional,range:0..1,...,extvalues:0"`
	CPLength          CPLength           `per:"ENUMERATED,range:0..1,...,extvalues:0"`
	PRSInfo           *PRSInfo           `per:",optional"`
}

type PRSBandwidth int64

const (
	PRSBandwidthN6   PRSBandwidth = 0
	PRSBandwidthN15  PRSBandwidth = 1
	PRSBandwidthN25  PRSBandwidth = 2
	PRSBandwidthN50  PRSBandwidth = 3
	PRSBandwidthN75  PRSBandwidth = 4
	PRSBandwidthN100 PRSBandwidth = 5
)

type NumDLFrames int64

const (
	NumDLFramesSF1   NumDLFrames = 0
	NumDLFramesSF2   NumDLFrames = 1
	NumDLFramesSF4   NumDLFrames = 2
	NumDLFramesSF6   NumDLFrames = 3
	NumDLFramesSFAdd NumDLFrames = 4
)

type PRSInfo struct {
	_                     [0]struct{}  `per:"extseq"`
	PRSBandwidth          PRSBandwidth `per:"ENUMERATED,range:0..5,...,extvalues:0"`
	PRSConfigurationIndex int64        `per:",range:0..4095"`
	NumDLFrames           NumDLFrames  `per:"ENUMERATED,range:0..3,...,extvalues:1"`
}

// =====================================================================
// OTDOA-NeighbourCellInfoList (TS 37.355 §6.5.1.2)
// =====================================================================

// OTDOA-NeighbourCellInfoList ::= SEQUENCE (SIZE (1..maxFreqLayers)) OF OTDOA-NeighbourFreqInfo
// maxFreqLayers INTEGER ::= 3
//
// OTDOA-NeighbourFreqInfo ::= SEQUENCE (SIZE (1..24)) OF OTDOA-NeighbourCellInfoElement
type OTDOANeighbourCellInfoList struct {
	List []OTDOANeighbourFreqInfo `per:"SEQUENCE-OF,size:1..3"`
}

type OTDOANeighbourFreqInfo struct {
	List []OTDOANeighbourCellInfoElement `per:"SEQUENCE-OF,size:1..24"`
}

// =====================================================================
// OTDOA-NeighbourCellInfoElement (TS 37.355 §6.5.1.2)
// =====================================================================

//	OTDOA-NeighbourCellInfoElement ::= SEQUENCE {
//	    physCellId				INTEGER (0..503),
//	    cellGlobalId			ECGI					OPTIONAL,
//	    earfcn					ARFCN-ValueEUTRA		OPTIONAL,
//	    cpLength				ENUMERATED {normal, extended, ...}	OPTIONAL,
//	    prsInfo					PRS-Info				OPTIONAL,
//	    antennaPortConfig		ENUMERATED {ports-1-or-2, ports-4, ...}	OPTIONAL,
//	    slotNumberOffset		INTEGER (0..19)			OPTIONAL,
//	    prs-SubframeOffset		INTEGER (0..1279)		OPTIONAL,
//	    expectedRSTD			INTEGER (0..16383),
//	    expectedRSTD-Uncertainty	INTEGER (0..1023),
//	    ...,
//	    [[
//	        earfcn-v9a0			ARFCN-ValueEUTRA-v9a0	OPTIONAL
//	    ]],
//	    [[
//	        tpId-r14				INTEGER (0..4095)				OPTIONAL,
//	        prs-only-tp-r14		ENUMERATED { true }		OPTIONAL,
//	        cpLengthCRS-r14		ENUMERATED { normal, extended, ... }	OPTIONAL,
//	        sameMBSFNconfigNeighbour-r14	BOOLEAN		OPTIONAL,
//	        dlBandwidth-r14		ENUMERATED {n6, n15, n25, n50, n75, n100}	OPTIONAL,
//	        addPRSconfigNeighbour-r14	SEQUENCE (SIZE (1..maxAddPRSconfig-r14)) OF Add-PRSconfigNeighbourElement-r14	OPTIONAL
//	    ]],
//	    [[
//	        tdd-config-v1520		TDD-Config-v1520		OPTIONAL
//	    ]]
//	}
type OTDOANeighbourCellInfoElement struct {
	_                       [0]struct{}        `per:"extseq"`
	PhysCellID              int64              `per:",range:0..503"`
	CellGlobalID            *ECGI              `per:",optional"`
	EARFCN                  *int64             `per:",optional,range:0..65535"`
	CPLength                *CPLength          `per:"ENUMERATED,optional,range:0..1,...,extvalues:0"`
	PRSInfo                 *PRSInfo           `per:",optional"`
	AntennaPortConfig       *AntennaPortConfig `per:"ENUMERATED,optional,range:0..1,...,extvalues:0"`
	SlotNumberOffset        *int64             `per:",optional,range:0..19"`
	PRSSubframeOffset       *int64             `per:",optional,range:0..1279"`
	ExpectedRSTD            int64              `per:",range:0..16383"`
	ExpectedRSTDUncertainty int64              `per:",range:0..1023"`
}
