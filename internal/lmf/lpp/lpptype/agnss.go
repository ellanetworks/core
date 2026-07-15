// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import (
	"fmt"

	"github.com/free5gc/aper"
)

// =====================================================================
// A-GNSS-RequestCapabilities (TS 37.355 §6.5.2.11)
// =====================================================================

//	A-GNSS-RequestCapabilities ::= SEQUENCE {
//	    gnss-SupportListReq    BOOLEAN,
//	    assistanceDataSupportListReq BOOLEAN,
//	    locationVelocityTypesReq  BOOLEAN,
//	    ...
//	}
type AGNSSRequestCapabilities struct {
	GnssSupportListReq           bool
	AssistanceDataSupportListReq bool
	LocationVelocityTypesReq     bool
}

// =====================================================================
// A-GNSS-ProvideCapabilities (TS 37.355 §6.5.2.9) — extensible SEQUENCE
// =====================================================================

type AGNSSProvideCapabilities struct {
	GnssSupportList           *GNSSSupportList `aper:"optional,valueExt"`
	AssistanceDataSupportList *struct{}        `aper:"optional"`
	LocationCoordinateTypes   *struct{}        `aper:"optional"`
	VelocityTypes             *struct{}        `aper:"optional"`
}

// =====================================================================
// GNSS-SupportList (TS 37.355 §6.5.2.9)
// =====================================================================

// GNSS-SupportList ::= SEQUENCE (SIZE(1..16)) OF GNSS-SupportElement
type GNSSSupportList struct {
	List []GNSSSupportElement `aper:"sizeLB:1,sizeUB:16"`
}

//	GNSS-SupportElement ::= SEQUENCE {
//	    gnss-ID       GNSS-ID,
//	    sbas-IDs      SBAS-IDs     OPTIONAL,
//	    agnss-Modes      PositioningModes,
//	    gnss-Signals     GNSS-SignalIDs,
//	    fta-MeasSupport     SEQUENCE { ... } OPTIONAL,
//	    adr-Support      BOOLEAN,
//	    velocityMeasurementSupport  BOOLEAN,
//	    ...,
//	    [[ ... ]]
//	}
type GNSSSupportElement struct {
	GnssID                     GNSSID           `aper:"valueExt"`
	SbasIDs                    *struct{}        `aper:"optional"`
	AGNSSModes                 PositioningModes `aper:"valueExt"`
	GnssSignals                GNSSSignalIDs    `aper:"valueExt"`
	FtaMeasSupport             *struct{}        `aper:"optional"`
	AdrSupport                 bool
	VelocityMeasurementSupport bool
}

// =====================================================================
// GNSS-ID (TS 37.355 §6.4.1)
// =====================================================================

//	GNSS-ID ::= SEQUENCE {
//	    gnss-id    ENUMERATED{ gps, sbas, qzss, galileo, glonass, ..., bds, navic-v1610 },
//	    ...
//	}
const (
	GnssIDGps     aper.Enumerated = 0
	GnssIDSbas    aper.Enumerated = 1
	GnssIDQzss    aper.Enumerated = 2
	GnssIDGalileo aper.Enumerated = 3
	GnssIDGlonass aper.Enumerated = 4
	GnssIDBds     aper.Enumerated = 5
	GnssIDNavic   aper.Enumerated = 6
)

type GNSSID struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:6,valueExt"`
}

// =====================================================================
// GNSS-ID-Bitmap (TS 37.355 §6.4.1)
// =====================================================================

//	GNSS-ID-Bitmap ::= SEQUENCE {
//	    gnss-ids   BIT STRING { gps(0), sbas(1), qzss(2), galileo(3), glonass(4), bds(5), navic-v1610(6) } (SIZE (1..16)),
//	    ...
//	}
type GNSSIDBitmap struct {
	GnssIDs aper.BitString `aper:"sizeLB:1,sizeUB:16"`
}

// =====================================================================
// PositioningModes (TS 37.355 §6.4.1)
// =====================================================================

//	PositioningModes ::= SEQUENCE {
//	    posModes  BIT STRING { standalone(0), ue-based(1), ue-assisted(2) } (SIZE (1..8)),
//	    ...
//	}
type PositioningModes struct {
	PosModes aper.BitString `aper:"sizeLB:1,sizeUB:8"`
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
	GnssSignalIDs aper.BitString `aper:"sizeLB:8,sizeUB:8"`
}

// =====================================================================
// A-GNSS-RequestLocationInformation (TS 37.355 §6.5.2.7)
// =====================================================================

//	A-GNSS-RequestLocationInformation ::= SEQUENCE {
//	    gnss-PositioningInstructions  GNSS-PositioningInstructions,
//	    ...
//	}
type AGNSSRequestLocationInformation struct {
	GnssPositioningInstructions GNSSPositioningInstructions `aper:"valueExt"`
}

// =====================================================================
// GNSS-PositioningInstructions (TS 37.355 §6.5.2.8)
// =====================================================================

//	GNSS-PositioningInstructions ::= SEQUENCE {
//	    gnss-Methods    GNSS-ID-Bitmap,
//	    fineTimeAssistanceMeasReq BOOLEAN,
//	    adrMeasReq     BOOLEAN,
//	    multiFreqMeasReq   BOOLEAN,
//	    assistanceAvailability  BOOLEAN,
//	    ...,
//	    [[ ... ]]
//	}
type GNSSPositioningInstructions struct {
	GnssMethods               GNSSIDBitmap `aper:"valueExt"`
	FineTimeAssistanceMeasReq bool
	AdrMeasReq                bool
	MultiFreqMeasReq          bool
	AssistanceAvailability    bool
}

// =====================================================================
// A-GNSS-ProvideLocationInformation (TS 37.355 §6.5.2.5)
// =====================================================================

//	A-GNSS-ProvideLocationInformation ::= SEQUENCE {
//	    gnss-SignalMeasurementInformation GNSS-SignalMeasurementInformation  OPTIONAL,
//	    gnss-LocationInformation   GNSS-LocationInformation    OPTIONAL,
//	    gnss-Error       A-GNSS-Error       OPTIONAL,
//	    ...
//	}
type AGNSSProvideLocationInformation struct {
	GnssSignalMeasurementInformation *struct{}                `aper:"optional"`
	GnssLocationInformation          *GNSSLocationInformation `aper:"optional,valueExt"`
	GnssError                        *AGNSSError              `aper:"optional"`
}

//	A-GNSS-Error ::= CHOICE {
//	    locationServerErrorCauses GNSS-LocationServerErrorCauses,
//	    targetDeviceErrorCauses   GNSS-TargetDeviceErrorCauses, ... }
const (
	AGNSSErrorPresentNothing int = iota
	AGNSSErrorPresentLocationServerErrorCauses
	AGNSSErrorPresentTargetDeviceErrorCauses
)

type AGNSSError struct {
	Present                 int
	TargetDeviceErrorCauses *GNSSTargetDeviceErrorCauses
}

//	GNSS-TargetDeviceErrorCauses ::= SEQUENCE {
//	    cause ENUMERATED { undefined, thereWereNotEnoughSatellitesReceived,
//	        assistanceDataMissing, notAllRequestedMeasurementsPossible, ... },
//	    fineTimeAssistanceMeasurementsNotPossible NULL OPTIONAL,
//	    adrMeasurementsNotPossible NULL OPTIONAL,
//	    multiFrequencyMeasurementsNotPossible NULL OPTIONAL, ... }
const (
	GNSSTargetDeviceErrorCausePresentUndefined                            aper.Enumerated = 0
	GNSSTargetDeviceErrorCausePresentThereWereNotEnoughSatellitesReceived aper.Enumerated = 1
	GNSSTargetDeviceErrorCausePresentAssistanceDataMissing                aper.Enumerated = 2
	GNSSTargetDeviceErrorCausePresentNotAllRequestedMeasurementsPossible  aper.Enumerated = 3
)

type GNSSTargetDeviceErrorCauses struct {
	Cause aper.Enumerated
}

// GNSSTargetDeviceErrorCauseString names an A-GNSS target device error cause.
func GNSSTargetDeviceErrorCauseString(c aper.Enumerated) string {
	switch c {
	case GNSSTargetDeviceErrorCausePresentUndefined:
		return "undefined"
	case GNSSTargetDeviceErrorCausePresentThereWereNotEnoughSatellitesReceived:
		return "thereWereNotEnoughSatellitesReceived"
	case GNSSTargetDeviceErrorCausePresentAssistanceDataMissing:
		return "assistanceDataMissing"
	case GNSSTargetDeviceErrorCausePresentNotAllRequestedMeasurementsPossible:
		return "notAllRequestedMeasurementsPossible"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

// =====================================================================
// GNSS-LocationInformation (TS 37.355 §6.5.2.6)
// =====================================================================

//	GNSS-LocationInformation ::= SEQUENCE {
//	    measurementReferenceTime  MeasurementReferenceTime,
//	    agnss-List      GNSS-ID-Bitmap,
//	    ...,
//	    [[ ... ]]
//	}
type GNSSLocationInformation struct {
	MeasurementReferenceTime MeasurementReferenceTime `aper:"valueExt"`
	AgnssList                GNSSIDBitmap             `aper:"valueExt"`
}

// =====================================================================
// MeasurementReferenceTime (TS 37.355 §6.5.2.6)
// =====================================================================

//	MeasurementReferenceTime ::= SEQUENCE {
//	    gnss-TOD-msec   INTEGER (0..3599999),
//	    gnss-TOD-frac   INTEGER (0..3999) OPTIONAL,
//	    gnss-TOD-unc   INTEGER (0..127) OPTIONAL,
//	    gnss-TimeID    GNSS-ID OPTIONAL,
//	    networkTime    NetworkTime OPTIONAL,
//	    ...
//	}
type MeasurementReferenceTime struct {
	GnssTODMsec int64     `aper:"valueLB:0,valueUB:3599999"`
	GnssTODFrac *int64    `aper:"optional,valueLB:0,valueUB:3999"`
	GnssTODUnc  *int64    `aper:"optional,valueLB:0,valueUB:127"`
	GnssTimeID  *GNSSID   `aper:"optional"`
	NetworkTime *struct{} `aper:"optional"`
}

// =====================================================================
// A-GNSS-ProvideAssistanceData (TS 37.355 §6.5.2.1)
// =====================================================================

//	A-GNSS-ProvideAssistanceData ::= SEQUENCE {
//	    gnss-CommonAssistData   GNSS-CommonAssistData    OPTIONAL,
//	    gnss-GenericAssistData   GNSS-GenericAssistData    OPTIONAL,
//	    gnss-Error      A-GNSS-Error      OPTIONAL,
//	    ...
//	}
type AGNSSProvideAssistanceData struct {
	GnssCommonAssistData  *struct{} `aper:"optional"`
	GnssGenericAssistData *struct{} `aper:"optional"`
	GnssError             *struct{} `aper:"optional"`
}
