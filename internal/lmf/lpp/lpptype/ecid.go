// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/per"

const (
	ECIDRequestedMeasurementsRSRP   = 0
	ECIDRequestedMeasurementsRSRQ   = 1
	ECIDRequestedMeasurementsUERxTx = 2
	ECIDRequestedMeasurementsNRSRP  = 3
	ECIDRequestedMeasurementsNRSRQ  = 4
)

type ECIDRequestLocationInformation struct {
	_                     [0]struct{} `per:"extseq"`
	RequestedMeasurements []bool      `per:",size:1..8"`
}

type ECIDProvideLocationInformation struct {
	_                                [0]struct{}                       `per:"extseq"`
	ECIDSignalMeasurementInformation *ECIDSignalMeasurementInformation `per:",optional"`
	ECIDError                        *ECIDError                        `per:",optional"`
}

type ECIDSignalMeasurementInformation struct {
	_                          [0]struct{}             `per:"extseq"`
	PrimaryCellMeasuredResults *MeasuredResultsElement `per:",optional"`
	MeasuredResultsList        MeasuredResultsList
}

type MeasuredResultsList struct {
	List []MeasuredResultsElement `per:"SEQUENCE-OF,size:1..32"`
}

type MeasuredResultsElement struct {
	_                 [0]struct{}               `per:"extseq"`
	PhysCellID        int64                     `per:",range:0..503"`
	CellGlobalID      *CellGlobalIdEUTRAAndUTRA `per:",optional"`
	ARFCNEUTRA        int64                     `per:",range:0..65535"`
	SystemFrameNumber []bool                    `per:",optional,size:10"`
	RSRPResult        *int64                    `per:",optional,range:0..97"`
	RSRQResult        *int64                    `per:",optional,range:0..34"`
	UERxTxTimeDiff    *int64                    `per:",optional,range:0..4095"`
}

type ECIDError struct {
	_                         [0]struct{}                    `per:"extseq"`
	LocationServerErrorCauses *ECIDLocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *ECIDTargetDeviceErrorCauses   `per:",choice:1,optional"`
}

const (
	ECIDLocationServerErrorCauseUndefined int64 = 0
)

type ECIDLocationServerErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:"ENUMERATED,range:0..0,..."`
}

const (
	ECIDTargetDeviceErrorCauseUndefined                           int64 = 0
	ECIDTargetDeviceErrorCauseRequestedMeasurementNotAvailable    int64 = 1
	ECIDTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible int64 = 2
)

type ECIDTargetDeviceErrorCauses struct {
	_                            [0]struct{} `per:"extseq"`
	Cause                        int64       `per:"ENUMERATED,range:0..2,..."`
	RSRPMeasurementNotPossible   *per.Null   `per:",optional"`
	RSRQMeasurementNotPossible   *per.Null   `per:",optional"`
	UERxTxMeasurementNotPossible *per.Null   `per:",optional"`
}
