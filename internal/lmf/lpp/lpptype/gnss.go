// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/per"

// =====================================================================
// GNSS-ID (TS 37.355 §6.4.1)
// =====================================================================

//	GNSS-ID ::= SEQUENCE {
//	    gnss-id    ENUMERATED{ gps, sbas, qzss, galileo, glonass, ..., bds, navic-v1610 },
//	    ...
//	}
type GNSSIDValue int64

const (
	GNSSIDGPS     GNSSIDValue = 0
	GNSSIDSBAS    GNSSIDValue = 1
	GNSSIDQZSS    GNSSIDValue = 2
	GNSSIDGalileo GNSSIDValue = 3
	GNSSIDGLONASS GNSSIDValue = 4
	GNSSIDBDS     GNSSIDValue = 5
	GNSSIDNavIC   GNSSIDValue = 6
)

type GNSSID struct {
	_      [0]struct{} `per:"extseq"`
	GNSSID GNSSIDValue `per:"ENUMERATED,range:0..4,...,extvalues:2"`
}

// =====================================================================
// GNSS-ID-Bitmap (TS 37.355 §6.4.1)
// =====================================================================

//	GNSS-ID-Bitmap ::= SEQUENCE {
//	    gnss-ids   BIT STRING { gps(0), sbas(1), qzss(2), galileo(3), glonass(4), bds(5), navic-v1610(6) } (SIZE (1..16)),
//	    ...
//	}
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

type SBASIDValue int64

const (
	SBASIDWAAS  SBASIDValue = 0
	SBASIDEGNOS SBASIDValue = 1
	SBASIDMSAS  SBASIDValue = 2
	SBASIDGAGAN SBASIDValue = 3
)

type SBASID struct {
	_      [0]struct{} `per:"extseq"`
	SBASID SBASIDValue `per:"ENUMERATED,range:0..3,...,extvalues:0"`
}

// =====================================================================
// PositioningModes (TS 37.355 §6.4.1)
// =====================================================================

//	PositioningModes ::= SEQUENCE {
//	    posModes  BIT STRING { standalone(0), ue-based(1), ue-assisted(2) } (SIZE (1..8)),
//	    ...
//	}
const (
	PosModesStandalone = 0
	PosModesUEBased    = 1
	PosModesUEAssisted = 2
)

type PositioningModes struct {
	_        [0]struct{} `per:"extseq"`
	PosModes []bool      `per:",size:1..8"`
}

// =====================================================================
// GNSS-SignalIDs (TS 37.355 §6.4.1)
// =====================================================================

//	GNSS-SignalIDs ::= SEQUENCE {
//	    gnss-SignalIDs  BIT STRING (SIZE(8)),
//	    ...,
//	    [[ gnss-SignalIDs-Ext-r15 BIT STRING (SIZE(16)) OPTIONAL ]]
//	}
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

type GNSSLocationServerErrorCause int64

const (
	GNSSLocationServerErrorCauseUndefined                                                                  GNSSLocationServerErrorCause = 0
	GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsNotSupportedByServer                            GNSSLocationServerErrorCause = 1
	GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsSupportedButCurrentlyNotAvailableByServer       GNSSLocationServerErrorCause = 2
	GNSSLocationServerErrorCauseUndeliveredAssistanceDataIsPartlyNotSupportedAndPartlyNotAvailableByServer GNSSLocationServerErrorCause = 3
	GNSSLocationServerErrorCauseUnconfirmedPeriodicAssistanceDataIsNotSupported                            GNSSLocationServerErrorCause = 4
	GNSSLocationServerErrorCauseUnconfirmedPeriodicAssistanceDataIsSupportedButCurrentlyNotAvailable       GNSSLocationServerErrorCause = 5
	GNSSLocationServerErrorCauseUnconfirmedPeriodicAssistanceDataIsPartlyNotSupportedAndPartlyNotAvailable GNSSLocationServerErrorCause = 6
	GNSSLocationServerErrorCauseUndeliveredPeriodicAssistanceDataIsCurrentlyNotAvailable                   GNSSLocationServerErrorCause = 7
)

type GNSSLocationServerErrorCauses struct {
	_     [0]struct{}                  `per:"extseq"`
	Cause GNSSLocationServerErrorCause `per:"ENUMERATED,range:0..3,...,extvalues:4"`
}

type GNSSTargetDeviceErrorCause int64

const (
	GNSSTargetDeviceErrorCauseUndefined                            GNSSTargetDeviceErrorCause = 0
	GNSSTargetDeviceErrorCauseThereWereNotEnoughSatellitesReceived GNSSTargetDeviceErrorCause = 1
	GNSSTargetDeviceErrorCauseAssistanceDataMissing                GNSSTargetDeviceErrorCause = 2
	GNSSTargetDeviceErrorCauseNotAllRequestedMeasurementsPossible  GNSSTargetDeviceErrorCause = 3
)

type GNSSTargetDeviceErrorCauses struct {
	_                                         [0]struct{}                `per:"extseq"`
	Cause                                     GNSSTargetDeviceErrorCause `per:"ENUMERATED,range:0..3,...,extvalues:0"`
	FineTimeAssistanceMeasurementsNotPossible *per.Null                  `per:",optional"`
	ADRMeasurementsNotPossible                *per.Null                  `per:",optional"`
	MultiFrequencyMeasurementsNotPossible     *per.Null                  `per:",optional"`
}

type AGNSSError struct {
	_                         [0]struct{}                    `per:"extseq"`
	LocationServerErrorCauses *GNSSLocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *GNSSTargetDeviceErrorCauses   `per:",choice:1,optional"`
}
