// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/per"

type NRECIDProvideLocationInformation struct {
	_                                  [0]struct{}                         `per:"extseq"`
	NRECIDSignalMeasurementInformation *NRECIDSignalMeasurementInformation `per:",optional"`
	NRECIDError                        *NRECIDError                        `per:",optional"`
}

type NRECIDSignalMeasurementInformation struct {
	_                            [0]struct{} `per:"extseq"`
	NRPrimaryCellMeasuredResults NRMeasuredResultsElement
	NRMeasuredResultsList        []NRMeasuredResultsElement `per:"SEQUENCE-OF,optional,size:1..32"`
}

type NRMeasuredResultsElement struct {
	_                   [0]struct{} `per:"extseq"`
	NRPhysCellID        int64       `per:",range:0..1007"`
	NRARFCN             NRMeasuredResultsElementARFCN
	NRCellGlobalID      *NCGI                  `per:",optional"`
	SystemFrameNumber   []bool                 `per:",optional,size:10"`
	ResultsSSBCell      *MeasQuantityResults   `per:",optional"`
	ResultsCSIRSCell    *MeasQuantityResults   `per:",optional"`
	ResultsSSBIndexes   []ResultsPerSSBIndex   `per:"SEQUENCE-OF,optional,size:1..64"`
	ResultsCSIRSIndexes []ResultsPerCSIRSIndex `per:"SEQUENCE-OF,optional,size:1..64"`
}

type NRMeasuredResultsElementARFCN struct {
	SSBARFCN    *int64 `per:",choice:0,optional,range:0..3279165"`
	CSIRSPointA *int64 `per:",choice:1,optional,range:0..3279165"`
}

type NCGI struct {
	MCC            []MCCMNCDigit `per:"SEQUENCE-OF,size:3"`
	MNC            []MCCMNCDigit `per:"SEQUENCE-OF,size:2..3"`
	NRCellIdentity []bool        `per:",size:36"`
}

type MeasQuantityResults struct {
	NRRSRP *int64 `per:",optional,range:0..127"`
	NRRSRQ *int64 `per:",optional,range:0..127"`
}

type ResultsPerSSBIndex struct {
	SSBIndex   int64 `per:",range:0..63"`
	SSBResults MeasQuantityResults
}

type ResultsPerCSIRSIndex struct {
	CSIRSIndex   int64 `per:",range:0..95"`
	CSIRSResults MeasQuantityResults
}

const (
	NRECIDRequestedMeasurementsSSRSRPReq  = 0
	NRECIDRequestedMeasurementsSSRSRQReq  = 1
	NRECIDRequestedMeasurementsCSIRSRPReq = 2
	NRECIDRequestedMeasurementsCSIRSRQReq = 3
)

type NRECIDRequestLocationInformation struct {
	_                     [0]struct{} `per:"extseq"`
	RequestedMeasurements []bool      `per:",size:1..8"`
}

const (
	NRECIDMeasSupportedSSRSRPSup  = 0
	NRECIDMeasSupportedSSRSRQSup  = 1
	NRECIDMeasSupportedCSIRSRPSup = 2
	NRECIDMeasSupportedCSIRSRQSup = 3
)

type NRECIDReportingSupport int64

const (
	NRECIDReportingSupportSupported NRECIDReportingSupport = 0
)

type NRECIDProvideCapabilities struct {
	_                   [0]struct{}             `per:"extseq"`
	NRECIDMeasSupported []bool                  `per:",size:1..8"`
	PeriodicalReporting *NRECIDReportingSupport `per:"ENUMERATED,optional,range:0..0"`
	TriggeredReporting  *NRECIDReportingSupport `per:"ENUMERATED,optional,range:0..0"`
}

type NRECIDRequestCapabilities struct {
	_ [0]struct{} `per:"extseq"`
}

type NRECIDError struct {
	_                         [0]struct{}                      `per:"extseq"`
	LocationServerErrorCauses *NRECIDLocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *NRECIDTargetDeviceErrorCauses   `per:",choice:1,optional"`
}

type NRECIDLocationServerErrorCause int64

const (
	NRECIDLocationServerErrorCauseUndefined NRECIDLocationServerErrorCause = 0
)

type NRECIDLocationServerErrorCauses struct {
	_     [0]struct{}                    `per:"extseq"`
	Cause NRECIDLocationServerErrorCause `per:"ENUMERATED,range:0..0,...,extvalues:0"`
}

type NRECIDTargetDeviceErrorCause int64

const (
	NRECIDTargetDeviceErrorCauseUndefined                           NRECIDTargetDeviceErrorCause = 0
	NRECIDTargetDeviceErrorCauseRequestedMeasurementNotAvailable    NRECIDTargetDeviceErrorCause = 1
	NRECIDTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible NRECIDTargetDeviceErrorCause = 2
)

type NRECIDTargetDeviceErrorCauses struct {
	_                             [0]struct{}                  `per:"extseq"`
	Cause                         NRECIDTargetDeviceErrorCause `per:"ENUMERATED,range:0..2,...,extvalues:0"`
	SSRSRPMeasurementNotPossible  *per.Null                    `per:",optional"`
	SSRSRQMeasurementNotPossible  *per.Null                    `per:",optional"`
	CSIRSRPMeasurementNotPossible *per.Null                    `per:",optional"`
	CSIRSRQMeasurementNotPossible *per.Null                    `per:",optional"`
}
