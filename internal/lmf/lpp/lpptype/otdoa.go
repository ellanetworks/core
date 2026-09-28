// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

type OTDOARequestLocationInformation struct {
	_                      [0]struct{} `per:"extseq"`
	AssistanceAvailability bool
}

type OTDOAProvideLocationInformation struct {
	_                                 [0]struct{}                        `per:"extseq"`
	OTDOASignalMeasurementInformation *OTDOASignalMeasurementInformation `per:",optional"`
	OTDOAError                        *OTDOAError                        `per:",optional"`
}

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

type NeighbourMeasurementElement struct {
	_                     [0]struct{} `per:"extseq"`
	PhysCellIDNeighbour   int64       `per:",range:0..503"`
	CellGlobalIDNeighbour *ECGI       `per:",optional"`
	EARFCNNeighbour       *int64      `per:",optional,range:0..65535"`
	RSTD                  int64       `per:",range:0..12711"`
	RSTDQuality           OTDOAMeasQuality
}

type OTDOAError struct {
	_                         [0]struct{}                     `per:"extseq"`
	LocationServerErrorCauses *OTDOALocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *OTDOATargetDeviceErrorCauses   `per:",choice:1,optional"`
}

const (
	OTDOALocationServerErrorCauseUndefined                                       int64 = 0
	OTDOALocationServerErrorCauseAssistanceDataNotSupportedByServer              int64 = 1
	OTDOALocationServerErrorCauseAssistanceDataSupportedButCurrentlyNotAvailable int64 = 2
)

type OTDOALocationServerErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:"ENUMERATED,range:0..2,..."`
}

const (
	OTDOATargetDeviceErrorCauseUndefined                                     int64 = 0
	OTDOATargetDeviceErrorCauseAssistanceDataMissing                         int64 = 1
	OTDOATargetDeviceErrorCauseUnableToMeasureReferenceCell                  int64 = 2
	OTDOATargetDeviceErrorCauseUnableToMeasureAnyNeighbourCell               int64 = 3
	OTDOATargetDeviceErrorCauseAttemptedButUnableToMeasureSomeNeighbourCells int64 = 4
)

type OTDOATargetDeviceErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:"ENUMERATED,range:0..4,..."`
}

type OTDOARequestAssistanceData struct {
	_          [0]struct{} `per:"extseq"`
	PhysCellID int64       `per:",range:0..503"`
}

type OTDOAProvideAssistanceData struct {
	_                      [0]struct{}                 `per:"extseq"`
	OTDOAReferenceCellInfo *OTDOAReferenceCellInfo     `per:",optional"`
	OTDOANeighbourCellInfo *OTDOANeighbourCellInfoList `per:",optional"`
	OTDOAError             *OTDOAError                 `per:",optional"`
}

const (
	AntennaPortConfigPorts1Or2 int64 = 0
	AntennaPortConfigPorts4    int64 = 1
)

const (
	CPLengthNormal   int64 = 0
	CPLengthExtended int64 = 1
)

type OTDOAReferenceCellInfo struct {
	_                 [0]struct{} `per:"extseq"`
	PhysCellID        int64       `per:",range:0..503"`
	CellGlobalID      *ECGI       `per:",optional"`
	EARFCNRef         *int64      `per:",optional,range:0..65535"`
	AntennaPortConfig *int64      `per:"ENUMERATED,optional,range:0..1,..."`
	CPLength          int64       `per:"ENUMERATED,range:0..1,..."`
	PRSInfo           *PRSInfo    `per:",optional"`
}

const (
	PRSBandwidthN6   int64 = 0
	PRSBandwidthN15  int64 = 1
	PRSBandwidthN25  int64 = 2
	PRSBandwidthN50  int64 = 3
	PRSBandwidthN75  int64 = 4
	PRSBandwidthN100 int64 = 5
)

const (
	NumDLFramesSF1 int64 = 0
	NumDLFramesSF2 int64 = 1
	NumDLFramesSF4 int64 = 2
	NumDLFramesSF6 int64 = 3
)

type PRSInfo struct {
	_                     [0]struct{} `per:"extseq"`
	PRSBandwidth          int64       `per:"ENUMERATED,range:0..5,..."`
	PRSConfigurationIndex int64       `per:",range:0..4095"`
	NumDLFrames           int64       `per:"ENUMERATED,range:0..3,..."`
}

type OTDOANeighbourCellInfoList struct {
	List []OTDOANeighbourFreqInfo `per:"SEQUENCE-OF,size:1..3"`
}

type OTDOANeighbourFreqInfo struct {
	List []OTDOANeighbourCellInfoElement `per:"SEQUENCE-OF,size:1..24"`
}

type OTDOANeighbourCellInfoElement struct {
	_                       [0]struct{} `per:"extseq"`
	PhysCellID              int64       `per:",range:0..503"`
	CellGlobalID            *ECGI       `per:",optional"`
	EARFCN                  *int64      `per:",optional,range:0..65535"`
	CPLength                *int64      `per:"ENUMERATED,optional,range:0..1,..."`
	PRSInfo                 *PRSInfo    `per:",optional"`
	AntennaPortConfig       *int64      `per:"ENUMERATED,optional,range:0..1,..."`
	SlotNumberOffset        *int64      `per:",optional,range:0..19"`
	PRSSubframeOffset       *int64      `per:",optional,range:0..1279"`
	ExpectedRSTD            int64       `per:",range:0..16383"`
	ExpectedRSTDUncertainty int64       `per:",range:0..1023"`
}
