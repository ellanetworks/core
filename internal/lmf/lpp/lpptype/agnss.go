// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/internal/per"

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
	_                            [0]struct{} `per:"extseq"`
	GnssSupportListReq           bool
	AssistanceDataSupportListReq bool
	LocationVelocityTypesReq     bool
}

// =====================================================================
// A-GNSS-ProvideCapabilities (TS 37.355 §6.5.2.9) — extensible SEQUENCE
// =====================================================================

type AGNSSProvideCapabilities struct {
	_                         [0]struct{}      `per:"extseq"`
	GnssSupportList           *GNSSSupportList `per:",optional"`
	AssistanceDataSupportList *per.Null        `per:",optional"`
	LocationCoordinateTypes   *per.Null        `per:",optional"`
	VelocityTypes             *per.Null        `per:",optional"`
}

// =====================================================================
// GNSS-SupportList (TS 37.355 §6.5.2.9)
// =====================================================================

// GNSS-SupportList ::= SEQUENCE (SIZE(1..16)) OF GNSS-SupportElement
type GNSSSupportList struct {
	List []GNSSSupportElement `per:"SEQUENCE-OF,size:1..16"`
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
	_                          [0]struct{} `per:"extseq"`
	GnssID                     GNSSID
	SbasIDs                    *per.Null `per:",optional"`
	AGNSSModes                 PositioningModes
	GnssSignals                GNSSSignalIDs
	FtaMeasSupport             *per.Null `per:",optional"`
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
	GnssIDGps     int64 = 0
	GnssIDSbas    int64 = 1
	GnssIDQzss    int64 = 2
	GnssIDGalileo int64 = 3
	GnssIDGlonass int64 = 4
	GnssIDBds     int64 = 5
	GnssIDNavic   int64 = 6
)

type GNSSID struct {
	_     [0]struct{} `per:"extseq"`
	Value int64       `per:",range:0..6,..."`
}

// =====================================================================
// GNSS-ID-Bitmap (TS 37.355 §6.4.1)
// =====================================================================

//	GNSS-ID-Bitmap ::= SEQUENCE {
//	    gnss-ids   BIT STRING { gps(0), sbas(1), qzss(2), galileo(3), glonass(4), bds(5), navic-v1610(6) } (SIZE (1..16)),
//	    ...
//	}
type GNSSIDBitmap struct {
	_       [0]struct{} `per:"extseq"`
	GnssIDs []bool      `per:",size:1..16"`
}

// =====================================================================
// PositioningModes (TS 37.355 §6.4.1)
// =====================================================================

//	PositioningModes ::= SEQUENCE {
//	    posModes  BIT STRING { standalone(0), ue-based(1), ue-assisted(2) } (SIZE (1..8)),
//	    ...
//	}
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
	GnssSignalIDs []bool      `per:",size:8..8"`
}

// =====================================================================
// A-GNSS-RequestLocationInformation (TS 37.355 §6.5.2.7)
// =====================================================================

//	A-GNSS-RequestLocationInformation ::= SEQUENCE {
//	    gnss-PositioningInstructions  GNSS-PositioningInstructions,
//	    ...
//	}
type AGNSSRequestLocationInformation struct {
	_                           [0]struct{} `per:"extseq"`
	GnssPositioningInstructions GNSSPositioningInstructions
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
	_                         [0]struct{} `per:"extseq"`
	GnssMethods               GNSSIDBitmap
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
	_                                [0]struct{}              `per:"extseq"`
	GnssSignalMeasurementInformation *per.Null                `per:",optional"`
	GnssLocationInformation          *GNSSLocationInformation `per:",optional"`
	GnssError                        *per.Null                `per:",optional"`
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
	_                        [0]struct{} `per:"extseq"`
	MeasurementReferenceTime MeasurementReferenceTime
	AgnssList                GNSSIDBitmap
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
	_           [0]struct{} `per:"extseq"`
	GnssTODMsec int64       `per:",range:0..3599999"`
	GnssTODFrac *int64      `per:",optional,range:0..3999"`
	GnssTODUnc  *int64      `per:",optional,range:0..127"`
	GnssTimeID  *GNSSID     `per:",optional"`
	NetworkTime *per.Null   `per:",optional"`
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
	_                     [0]struct{} `per:"extseq"`
	GnssCommonAssistData  *per.Null   `per:",optional"`
	GnssGenericAssistData *per.Null   `per:",optional"`
	GnssError             *per.Null   `per:",optional"`
}
