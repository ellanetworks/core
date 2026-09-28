// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/per"

const (
	GNSSIDGPS     int64 = 0
	GNSSIDSBAS    int64 = 1
	GNSSIDQZSS    int64 = 2
	GNSSIDGalileo int64 = 3
	GNSSIDGLONASS int64 = 4
	GNSSIDBDS     int64 = 5
	GNSSIDNavIC   int64 = 6
)

type GNSSID struct {
	_      [0]struct{} `per:"extseq"`
	GNSSID int64       `per:"ENUMERATED,range:0..4,..."`
}

const (
	GNSSIDBitmapGPS     = 0
	GNSSIDBitmapSBAS    = 1
	GNSSIDBitmapQZSS    = 2
	GNSSIDBitmapGalileo = 3
	GNSSIDBitmapGLONASS = 4
	GNSSIDBitmapBDS     = 5
	GNSSIDBitmapNavIC   = 6
)

type GNSSIDBitmap struct {
	_       [0]struct{} `per:"extseq"`
	GNSSIDs []bool      `per:",size:1..16"`
}

const (
	SBASIDsWAAS  = 0
	SBASIDsEGNOS = 1
	SBASIDsMSAS  = 2
	SBASIDsGAGAN = 3
)

type SBASIDs struct {
	_       [0]struct{} `per:"extseq"`
	SBASIDs []bool      `per:",size:1..8"`
}

const (
	SBASIDWAAS  int64 = 0
	SBASIDEGNOS int64 = 1
	SBASIDMSAS  int64 = 2
	SBASIDGAGAN int64 = 3
)

type SBASID struct {
	_      [0]struct{} `per:"extseq"`
	SBASID int64       `per:"ENUMERATED,range:0..3,..."`
}

const (
	PosModesStandalone = 0
	PosModesUEBased    = 1
	PosModesUEAssisted = 2
)

type PositioningModes struct {
	_        [0]struct{} `per:"extseq"`
	PosModes []bool      `per:",size:1..8"`
}

type GNSSSignalIDs struct {
	_             [0]struct{} `per:"extseq"`
	GNSSSignalIDs []bool      `per:",size:8"`
}

type GNSSSignalID struct {
	_            [0]struct{} `per:"extseq"`
	GNSSSignalID int64       `per:",range:0..7"`
}

const (
	AccessTypesEUTRA = 0
	AccessTypesUTRA  = 1
	AccessTypesGSM   = 2
	AccessTypesNBIoT = 3
	AccessTypesNR    = 4
)

type AccessTypes struct {
	_           [0]struct{} `per:"extseq"`
	AccessTypes []bool      `per:",size:1..8"`
}

type SVID struct {
	_           [0]struct{} `per:"extseq"`
	SatelliteID int64       `per:",range:0..63"`
}

const (
	GNSSLocationServerErrorCauseUndefined                                                          int64 = 0
	GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsNotSupportedByServer                    int64 = 1
	GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsSupportedButCurrentlyNotAvailable       int64 = 2
	GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsPartlyNotSupportedAndPartlyNotAvailable int64 = 3
)

type GNSSLocationServerErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:"ENUMERATED,range:0..3,..."`
}

const (
	GNSSTargetDeviceErrorCauseUndefined                            int64 = 0
	GNSSTargetDeviceErrorCauseThereWereNotEnoughSatellitesReceived int64 = 1
	GNSSTargetDeviceErrorCauseAssistanceDataMissing                int64 = 2
	GNSSTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible  int64 = 3
)

type GNSSTargetDeviceErrorCauses struct {
	_                                         [0]struct{} `per:"extseq"`
	Cause                                     int64       `per:"ENUMERATED,range:0..3,..."`
	FineTimeAssistanceMeasurementsNotPossible *per.Null   `per:",optional"`
	ADRMeasurementsNotPossible                *per.Null   `per:",optional"`
	MultiFrequencyMeasurementsNotPossible     *per.Null   `per:",optional"`
}

type AGNSSError struct {
	_                         [0]struct{}                    `per:"extseq"`
	LocationServerErrorCauses *GNSSLocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *GNSSTargetDeviceErrorCauses   `per:",choice:1,optional"`
}
