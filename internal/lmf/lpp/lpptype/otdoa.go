// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

// =====================================================================
// OTDOA-Error (TS 37.355 §6.5.1.9)
// =====================================================================

//	OTDOA-Error ::= CHOICE {
//	    locationServerErrorCauses	OTDOA-LocationServerErrorCauses,
//	    targetDeviceErrorCauses	OTDOA-TargetDeviceErrorCauses,
//	    ...
//	}
type OTDOAError struct {
	_                         [0]struct{}                    `per:"extseq"`
	LocationServerErrorCauses *OTDALocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *OTDATargetDeviceErrorCauses   `per:",choice:1,optional"`
}

//	OTDOA-LocationServerErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, assistanceDataNotSupportedByServer,
//	                       assistanceDataSupportedButCurrentlyNotAvailableByServer, ... },
//	    ...
//	}
const (
	OTDALocationServerErrorCausesUndefined                              int64 = 0
	OTDALocationServerErrorCausesAssistanceDataNotSupportedByServer     int64 = 1
	OTDALocationServerErrorCausesAssistanceDataSupportedButNotAvailable int64 = 2
)

type OTDALocationServerErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:",range:0..2,..."`
}

//	OTDOA-TargetDeviceErrorCauses ::= SEQUENCE {
//	    cause	ENUMERATED { undefined, assistance-data-missing,
//	                       unableToMeasureReferenceCell, unableToMeasureAnyNeighbourCell,
//	                       attemptedButUnableToMeasureSomeNeighbourCells, ... },
//	    ...
//	}
const (
	OTDATargetDeviceErrorCausesUndefined                                     int64 = 0
	OTDATargetDeviceErrorCausesAssistanceDataMissing                         int64 = 1
	OTDATargetDeviceErrorCausesUnableToMeasureReferenceCell                  int64 = 2
	OTDATargetDeviceErrorCausesUnableToMeasureAnyNeighbourCell               int64 = 3
	OTDATargetDeviceErrorCausesAttemptedButUnableToMeasureSomeNeighbourCells int64 = 4
)

type OTDATargetDeviceErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:",range:0..4,..."`
}

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
	MultipathRSTD          *int64 `per:",optional,range:0..0,..."`
	MaxNoOfRSTDmeas        *int64 `per:",optional,range:1..32"`
	MotionMeasurements     *int64 `per:",optional,range:0..0,..."`
}

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
type ECIDRequestLocationInformation struct {
	_                     [0]struct{} `per:"extseq"`
	RequestedMeasurements []bool      `per:",size:1..8"`
}

// =====================================================================
// EPDU types (TS 37.355 §6.7)
// =====================================================================

// EPDU-Sequence ::= SEQUENCE (SIZE (1..maxEPDU)) OF EPDU
// maxEPDU INTEGER ::= 16
type EPDUSequence struct {
	List []EPDU `per:"SEQUENCE-OF,size:1..16"`
}

//	EPDU ::= SEQUENCE {
//	    ePDU-Identifier			EPDU-Identifier,
//	    ePDU-Body				EPDU-Body
//	}
type EPDU struct {
	EPDUIdentifier EPDUIdentifier
	EPDUBody       []byte `per:",size:0..65535"`
}

//	EPDU-Identifier ::= SEQUENCE {
//	    ePDU-ID					EPDU-ID,
//	    ePDU-Name				EPDU-Name		OPTIONAL,
//	    ...
//	}
//
// EPDU-ID ::= INTEGER (1..256)
// EPDU-Name ::= VisibleString (SIZE (1..32))
type EPDUIdentifier struct {
	_        [0]struct{} `per:"extseq"`
	EPDUID   int64       `per:",range:1..256"`
	EPDUName *string     `per:",optional,size:1..32"`
}

//	EPDU-Body ::= OCTET STRING
// (encoded as []byte in EPDU struct)

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
	_                           [0]struct{}                      `per:"extseq"`
	OtdoaReferenceCellInfo      *OTDOAReferenceCellInfo          `per:",optional"`
	OtdoaNeighbourCellInfo      *OTDOANeighbourCellInfoList      `per:",optional"`
	OtdoaError                  *OTDOAError                      `per:",optional"`
	OtdoaReferenceCellInfoNBR14 *OTDOAReferenceCellInfoNBR14     `per:",optional"`
	OtdoaNeighbourCellInfoNBR14 *OTDOANeighbourCellInfoListNBR14 `per:",optional"`
}

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
	_                        [0]struct{}          `per:"extseq"`
	PhysCellId               int64                `per:",range:0..503"`
	CellGlobalId             *ECGI                `per:",optional"`
	EarfcnRef                *ARFCNValueEUTRA     `per:",optional"`
	AntennaPortConfig        *int64               `per:",optional,range:0..1,..."`
	CpLength                 int64                `per:",range:0..1,..."`
	PrsInfo                  *PRSInfo             `per:",optional"`
	EarfcnRefV9a0            *ARFCNValueEUTRAV9a0 `per:",optional"`
	TpIdR14                  *int64               `per:",optional,range:0..4095"`
	CpLengthCRSR14           *int64               `per:",optional,range:0..1,..."`
	SameMBSFNconfigRefR14    *bool                `per:",optional"`
	DlBandwidthR14           *int64               `per:",optional,range:0..5,..."`
	AddPRSconfigRefR14       *[]PRSInfo           `per:",optional"`
	NrLTEsfnOffsetR15        *int64               `per:",optional,range:0..1023"`
	TddConfigV1520           *TDDConfigV1520      `per:",optional"`
	NrLTEfineTimingOffsetR15 *int64               `per:",optional,range:0..19"`
}

// =====================================================================
// OTDOA-NeighbourCellInfoList (TS 37.355 §6.5.1.2)
// =====================================================================

// OTDOA-NeighbourCellInfoList ::= SEQUENCE (SIZE (1..maxFreqLayers)) OF OTDOA-NeighbourFreqInfo
// maxFreqLayers INTEGER ::= 2
//
// OTDOA-NeighbourFreqInfo ::= SEQUENCE (SIZE (1..24)) OF OTDOA-NeighbourCellInfoElement
type OTDOANeighbourCellInfoList struct {
	List []OTDOANeighbourFreqInfo `per:"SEQUENCE-OF,size:1..2"`
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
//	        addPRSconfigNeighbour-r14	SEQUENCE (SIZE (1..maxAddPRSconfig-r14)) OF PRS-Info	OPTIONAL
//	    ]]
//	}
type OTDOANeighbourCellInfoElement struct {
	_                           [0]struct{}          `per:"extseq"`
	PhysCellId                  int64                `per:",range:0..503"`
	CellGlobalId                *ECGI                `per:",optional"`
	Earfcn                      *ARFCNValueEUTRA     `per:",optional"`
	CpLength                    *int64               `per:",optional,range:0..1,..."`
	PrsInfo                     *PRSInfo             `per:",optional"`
	AntennaPortConfig           *int64               `per:",optional,range:0..1,..."`
	SlotNumberOffset            *int64               `per:",optional,range:0..19"`
	PrsSubframeOffset           *int64               `per:",optional,range:0..1279"`
	ExpectedRSTD                int64                `per:",range:0..16383"`
	ExpectedRSTDUncertainty     int64                `per:",range:0..1023"`
	EarfcnV9a0                  *ARFCNValueEUTRAV9a0 `per:",optional"`
	TpIdR14                     *int64               `per:",optional,range:0..4095"`
	PrsOnlyTpR14                *int64               `per:",optional,range:0..0,..."`
	CpLengthCRSR14              *int64               `per:",optional,range:0..1,..."`
	SameMBSFNconfigNeighbourR14 *bool                `per:",optional"`
	DlBandwidthR14              *int64               `per:",optional,range:0..5,..."`
	AddPRSconfigNeighbourR14    *[]PRSInfo           `per:",optional"`
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
	_               [0]struct{} `per:"extseq"`
	PhysCellId      int64       `per:",range:0..503"`
	AdTypeR14       *[]bool     `per:",optional,size:1..8"`
	NrPhysCellIdR15 *int64      `per:",optional,range:0..1007"`
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
	_                         [0]struct{}                   `per:"extseq"`
	SystemFrameNumber         []bool                        `per:",size:10"`
	PhysCellIdRef             int64                         `per:",range:0..503"`
	CellGlobalIdRef           *ECGI                         `per:",optional"`
	EarfcnRef                 *ARFCNValueEUTRA              `per:",optional"`
	ReferenceQuality          *OTDOAMeasQuality             `per:",optional"`
	NeighbourMeasurementList  []NeighbourMeasurementElement `per:"SEQUENCE-OF,size:1..24"`
	EarfcnRefV9a0             *ARFCNValueEUTRAV9a0          `per:",optional"`
	TpIdRefR14                *int64                        `per:",optional,range:0..4095"`
	PrsIdRefR14               *int64                        `per:",optional,range:0..4095"`
	AdditionalPathsRefR14     *AdditionalPathListR14        `per:",optional"`
	NprsIdRefR14              *int64                        `per:",optional,range:0..4095"`
	CarrierFreqOffsetNBRefR14 *CarrierFreqOffsetNBR14       `per:",optional"`
	HyperSFNR14               *[]bool                       `per:",optional,size:10"`
	MotionTimeSourceR15       *MotionTimeSourceR15          `per:",optional"`
}

//	NeighbourMeasurementElement ::= SEQUENCE {
//	    physCellIdNeighbour		INTEGER (0..503),
//	    expectedRSTD			INTEGER (0..16383),
//	    expectedRSTD-Uncertainty	INTEGER (0..1023),
//	    rstd-UncertaintyMeasure	INTEGER (0..1023)	OPTIONAL,
//	    ...,
//	    [[
//	        rstdMeasure-r14		RSTD-Measure-r14	OPTIONAL
//	    ]]
//	}
type NeighbourMeasurementElement struct {
	_                       [0]struct{}     `per:"extseq"`
	PhysCellIdNeighbour     int64           `per:",range:0..503"`
	ExpectedRSTD            int64           `per:",range:0..16383"`
	ExpectedRSTDUncertainty int64           `per:",range:0..1023"`
	RstdUncertaintyMeasure  *int64          `per:",optional,range:0..1023"`
	RstdMeasureR14          *RSTDMeasureR14 `per:",optional"`
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
	_                                      [0]struct{}                             `per:"extseq"`
	OtdoaSignalMeasurementInformation      *OTDOASignalMeasurementInformation      `per:",optional"`
	OtdoaError                             *OTDOAError                             `per:",optional"`
	OtdoaSignalMeasurementInformationNBR14 *OTDOASignalMeasurementInformationNBR14 `per:",optional"`
}

// =====================================================================
// OTDOA-MeasQuality (TS 37.355 §6.5.1)
// =====================================================================

const (
	OTDOAMeasQualityGood   int64 = 0
	OTDOAMeasQualityMedium int64 = 1
	OTDOAMeasQualityPoor   int64 = 2
)

type OTDOAMeasQuality struct {
	Value int64 `per:",range:0..2,..."`
}

// =====================================================================
// CarrierFreqOffsetNB-r14 (TS 37.355 §6.5.1)
// =====================================================================

type CarrierFreqOffsetNBR14 int64

// =====================================================================
// ARFCN-ValueEUTRA (TS 37.355 §6.5.1)
// =====================================================================

type ARFCNValueEUTRA int64

// =====================================================================
// ARFCN-ValueEUTRA-v9a0 (TS 37.355 §6.5.1)
// =====================================================================

type ARFCNValueEUTRAV9a0 int64

// =====================================================================
// ARFCN-ValueEUTRA-r14 (TS 37.355 §6.5.1.2)
// =====================================================================

type ARFCNValueEUTRAR14 int64

// =====================================================================
// CarrierFreq-NB-r14 (TS 37.355 §6.5.1.2)
// =====================================================================

type CarrierFreqNBR14 int64

// =====================================================================
// MotionTimeSource-r15 (TS 37.355 §6.5.1)
// =====================================================================

const (
	MotionTimeSourceR15GNSS    int64 = 0
	MotionTimeSourceR15Network int64 = 1
)

type MotionTimeSourceR15 struct {
	Value int64 `per:",range:0..1,..."`
}

// =====================================================================
// RSTD-Measure-r14 (TS 37.355 §6.5.1)
// =====================================================================

type RSTDMeasureR14 struct {
	RSTD            int64 `per:",range:0..16383"`
	RSTDUncertainty int64 `per:",range:0..1023"`
}

// =====================================================================
// AdditionalPathElement-r14 (TS 37.355 §6.5.1)
// =====================================================================

type AdditionalPathElementR14 struct {
	_                      [0]struct{} `per:"extseq"`
	PhysCellId             int64       `per:",range:0..503"`
	RSTDMeasure            RSTDMeasureR14
	RSTDUncertaintyMeasure *int64 `per:",optional,range:0..1023"`
}

// =====================================================================
// AdditionalPathList-r14 (TS 37.355 §6.5.1)
// =====================================================================

type AdditionalPathListR14 struct {
	List []AdditionalPathElementR14 `per:"SEQUENCE-OF,size:1..8"`
}

// =====================================================================
// PRS-Info (TS 37.355 §6.5.1.2)
// =====================================================================

type PRSInfo struct {
	_                     [0]struct{}          `per:"extseq"`
	PRSConfigurationIndex int64                `per:",range:0..1279"`
	PRSBandwidth          int64                `per:",range:0..12"`
	PRSCellIdPRS          int64                `per:",range:0..503"`
	SubframeOffsetPRS     *int64               `per:",optional,range:0..15"`
	SubframeOffsetListPRS *int64               `per:",optional,range:0..63"`
	PRSActivationTimePRS  *[]bool              `per:",optional,size:14"`
	NumberOfPRSResources  *int64               `per:",optional,range:0..7"`
	AdditionalPRSConfig   *AdditionalPRSConfig `per:",optional"`
}

type AdditionalPRSConfig struct {
	_              [0]struct{} `per:"extseq"`
	AddNumDLFrames *int64      `per:",optional,range:0..7"`
	PRSOccGroupLen *int64      `per:",optional,range:0..7,..."`
	PRSHoppingInfo *int64      `per:",optional,range:0..0,..."`
}

// =====================================================================
// PRS-Info-NB-r14 (TS 37.355 §6.5.1.2)
// =====================================================================

type PRSInfoNBR14 struct {
	List []NPRSInfoR14 `per:"SEQUENCE-OF,size:1..5"`
}

type NPRSInfoR14 struct {
	_                     [0]struct{}       `per:"extseq"`
	OperationModeInfoNPRS int64             `per:",range:0..1,..."`
	NPRSCarrier           *CarrierFreqNBR14 `per:",optional"`
	NPRSSequenceInfo      *int64            `per:",optional,range:0..174"`
	NPRSID                *int64            `per:",optional,range:0..4095"`
	PartA                 *NPRSInfoPartAR14 `per:",optional"`
	PartB                 *NPRSInfoPartBR14 `per:",optional"`
}

type NPRSInfoPartAR14 struct {
	_                 [0]struct{} `per:"extseq"`
	SubframePattern10 *[]bool     `per:",optional,size:10"`
	SubframePattern40 *[]bool     `per:",optional,size:40"`
	PO2               *[]bool     `per:",optional,size:2"`
	PO4               *[]bool     `per:",optional,size:4"`
	PO8               *[]bool     `per:",optional,size:8"`
	PO16              *[]bool     `per:",optional,size:16"`
}

type NPRSInfoPartBR14 struct {
	_                   [0]struct{} `per:"extseq"`
	NPRSPeriod          int64       `per:",range:0..4,..."`
	NPRSStartSF         int64       `per:",range:0..2,..."`
	NPRSNumSF           int64       `per:",range:0..4,..."`
	NPRSMutingInfoBPO2  *[]bool     `per:",optional,size:2"`
	NPRSMutingInfoBPO4  *[]bool     `per:",optional,size:4"`
	NPRSMutingInfoBPO8  *[]bool     `per:",optional,size:8"`
	NPRSMutingInfoBPO16 *[]bool     `per:",optional,size:16"`
}

// =====================================================================
// OTDOA-ReferenceCellInfoNB-r14 (TS 37.355 §6.5.1.2)
// =====================================================================

type OTDOAReferenceCellInfoNBR14 struct {
	_                      [0]struct{}         `per:"extseq"`
	PhysCellIdNB           *int64              `per:",optional,range:0..503"`
	CellGlobalIdNB         *ECGI               `per:",optional"`
	CarrierFreq            *CarrierFreqNBR14   `per:",optional"`
	Earfcn                 *ARFCNValueEUTRAR14 `per:",optional"`
	EutraNumCRSPorts       *int64              `per:",optional,range:0..1,..."`
	OtdoaSIB1NBRepetitions *int64              `per:",optional,range:0..1,..."`
	NPRSInfo               *PRSInfoNBR14       `per:",optional"`
	NPRSSlotNumberOffset   *int64              `per:",optional,range:0..19"`
	NPRSSFNOffset          *int64              `per:",optional,range:0..63"`
	NPRSSubframeOffset     *int64              `per:",optional,range:0..1279"`
}

// =====================================================================
// OTDOA-NeighbourCellInfoNB-r14 (TS 37.355 §6.5.1.2)
// =====================================================================

type OTDOANeighbourCellInfoNBR14 struct {
	_                       [0]struct{}         `per:"extseq"`
	PhysCellIdNB            *int64              `per:",optional,range:0..503"`
	CellGlobalIdNB          *ECGI               `per:",optional"`
	CarrierFreq             *CarrierFreqNBR14   `per:",optional"`
	Earfcn                  *ARFCNValueEUTRAR14 `per:",optional"`
	EutraNumCRSPorts        *int64              `per:",optional,range:0..1,..."`
	OtdoaSIB1NBRepetitions  *int64              `per:",optional,range:0..1,..."`
	NPRSInfo                *PRSInfoNBR14       `per:",optional"`
	NPRSSlotNumberOffset    *int64              `per:",optional,range:0..19"`
	NPRSSFNOffset           *int64              `per:",optional,range:0..63"`
	NPRSSubframeOffset      *int64              `per:",optional,range:0..1279"`
	ExpectedRSTD            *int64              `per:",optional,range:0..16383"`
	ExpectedRSTDUncertainty *int64              `per:",optional,range:0..1023"`
	PRSNeighbourCellIndex   *int64              `per:",optional,range:1..72"`
}

type OTDOANeighbourCellInfoListNBR14 struct {
	List []OTDOANeighbourCellInfoNBR14 `per:"SEQUENCE-OF,size:1..72"`
}

// =====================================================================
// OTDOA-SignalMeasurementInformation-NB-r14 (TS 37.355 §6.5.1.5)
// =====================================================================

type OTDOASignalMeasurementInformationNBR14 struct {
	_                          [0]struct{}                        `per:"extseq"`
	SystemFrameNumber          []bool                             `per:",size:10"`
	PhysCellIdRefNB            int64                              `per:",range:0..503"`
	CellGlobalIdRefNB          *ECGI                              `per:",optional"`
	EarfcnRefNB                *ARFCNValueEUTRA                   `per:",optional"`
	ReferenceQuality           *OTDOAMeasQuality                  `per:",optional"`
	NeighbourMeasurementListNB []NeighbourMeasurementElementNBR14 `per:"SEQUENCE-OF,size:1..24"`
}

type NeighbourMeasurementElementNBR14 struct {
	_                       [0]struct{} `per:"extseq"`
	PhysCellIdNeighbourNB   int64       `per:",range:0..503"`
	ExpectedRSTD            int64       `per:",range:0..16383"`
	ExpectedRSTDUncertainty int64       `per:",range:0..1023"`
	RSTDUncertaintyMeasure  *int64      `per:",optional,range:0..1023"`
}

type NeighbourMeasurementListNBR14 struct {
	List []NeighbourMeasurementElementNBR14 `per:"SEQUENCE-OF,size:1..24"`
}

// =====================================================================
// TDD-Config-v1520 (TS 37.355 §6.5.1.2)
// =====================================================================

type TDDConfigV1520 struct {
	_                       [0]struct{} `per:"extseq"`
	SubframeAssignmentV1520 int64       `per:",range:0..6,..."`
}

// =====================================================================
// ECGI (TS 37.355 §6.5.1)
// =====================================================================

type ECGI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	TACAndECID   ECGITACAndECID
}

type PLMNIdentity struct {
	_   [0]struct{} `per:"extseq"`
	MCC Mcc
	MNC Mnc
}

type Mcc struct {
	List []MccValue `per:"SEQUENCE-OF,size:1..3"`
}

type MccValue struct {
	Value int64 `per:",range:0..9"`
}

type Mnc struct {
	List []MncValue `per:"SEQUENCE-OF,size:1..3"`
}

type MncValue struct {
	Value int64 `per:",range:0..9"`
}

type ECGITACAndECID struct {
	TAC               int64 `per:",range:0..63"`
	UTRANCellIdentity int64 `per:",range:0..1099511627775"`
}
