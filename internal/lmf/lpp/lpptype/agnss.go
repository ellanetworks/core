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
	_                         [0]struct{}                `per:"extseq"`
	GnssSupportList           *GNSSSupportList           `per:",optional"`
	AssistanceDataSupportList *AssistanceDataSupportList `per:",optional"`
	LocationCoordinateTypes   *LocationCoordinateTypes   `per:",optional"`
	VelocityTypes             *VelocityTypes             `per:",optional"`
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
	SbasIDs                    *SBASIDs `per:",optional"`
	AGNSSModes                 PositioningModes
	GnssSignals                GNSSSignalIDs
	FtaMeasSupport             *FTAMeasSupport `per:",optional"`
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
	_                                [0]struct{}                       `per:"extseq"`
	GnssSignalMeasurementInformation *GNSSSignalMeasurementInformation `per:",optional"`
	GnssLocationInformation          *GNSSLocationInformation          `per:",optional"`
	GnssError                        *GNSSError                        `per:",optional"`
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
//	    gnss-TimeID    GNSS-ID,
//	    networkTime    NetworkTime OPTIONAL,
//	    ...
//	}
type MeasurementReferenceTime struct {
	_           [0]struct{} `per:"extseq"`
	GnssTODMsec int64       `per:",range:0..3599999"`
	GnssTODFrac *int64      `per:",optional,range:0..3999"`
	GnssTODUnc  *int64      `per:",optional,range:0..127"`
	GnssTimeID  GNSSID
	NetworkTime *NetworkTime `per:",optional"`
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
	_                     [0]struct{}            `per:"extseq"`
	GnssCommonAssistData  *GNSSCommonAssistData  `per:",optional"`
	GnssGenericAssistData *GNSSGenericAssistData `per:",optional"`
	GnssError             *GNSSError             `per:",optional"`
}

// =====================================================================
// A-GNSS-RequestAssistanceData (TS 37.355 §6.5.2.3)
// =====================================================================

//	A-GNSS-RequestAssistanceData ::= SEQUENCE {
//	    gnss-CommonAssistDataReq GNSS-CommonAssistDataReq OPTIONAL,
//	    gnss-GenericAssistDataReq GNSS-GenericAssistDataReq OPTIONAL,
//	    ...,
//	    [[ gnss-PeriodicAssistDataReq-r15 GNSS-PeriodicAssistDataReq-r15 OPTIONAL ]]
//	}
type AGNSSRequestAssistanceData struct {
	_                            [0]struct{}               `per:"extseq"`
	GnssCommonAssistDataReq      *GNSSCommonAssistDataReq  `per:",optional"`
	GnssGenericAssistDataReq     *GNSSGenericAssistDataReq `per:",optional"`
	GnssPeriodicAssistDataReqR15 *per.Null                 `per:",optional"`
}

// =====================================================================
// GNSS-ReferenceTime (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSReferenceTime struct {
	_                         [0]struct{} `per:"extseq"`
	GnssSystemTime            GNSSSystemTime
	ReferenceTimeUnc          *int64                         `per:",optional,range:0..127"`
	GnssReferenceTimeForCells *[]GNSSReferenceTimeForOneCell `per:",optional"`
}

type GNSSReferenceTimeForOneCell struct {
	_                [0]struct{} `per:"extseq"`
	NetworkTime      NetworkTime
	ReferenceTimeUnc int64  `per:",range:0..127"`
	BsAlign          *int64 `per:",optional,range:0..0,..."`
}

// =====================================================================
// GNSS-ReferenceLocation (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSReferenceLocation struct {
	_              [0]struct{} `per:"extseq"`
	ThreeDLocation EllipsoidPointWithAltitudeAndUncertaintyEllipsoid
}

// =====================================================================
// GNSS-IonosphericModel (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSIonosphericModel struct {
	_         [0]struct{}                    `per:"extseq"`
	Klobuchar *GNSSIonosphericModelKlobuchar `per:",optional"`
}

type GNSSIonosphericModelKlobuchar struct {
	_      [0]struct{} `per:"extseq"`
	A0     *int64      `per:",optional,range:-7..7"`
	A1     *int64      `per:",optional,range:-7..7"`
	A2     *int64      `per:",optional,range:-15..15"`
	A3     *int64      `per:",optional,range:-15..15"`
	B0     *int64      `per:",optional,range:14..31"`
	B1     *int64      `per:",optional,range:14..31"`
	B2     *int64      `per:",optional,range:14..31"`
	B3     *int64      `per:",optional,range:14..31"`
	Alpha1 *int64      `per:",optional,range:-7..7"`
	Alpha2 *int64      `per:",optional,range:-7..7"`
	Alpha3 *int64      `per:",optional,range:-7..7"`
	Alpha4 *int64      `per:",optional,range:-7..7"`
	Beta1  *int64      `per:",optional,range:14..31"`
	Beta2  *int64      `per:",optional,range:14..31"`
	Beta3  *int64      `per:",optional,range:14..31"`
	Beta4  *int64      `per:",optional,range:14..31"`
}

// =====================================================================
// GNSS-EarthOrientationParameters (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSEarthOrientationParameters struct {
	_      [0]struct{} `per:"extseq"`
	UT1UTC *int64      `per:",optional,range:-127..127"`
}

// =====================================================================
// GNSS-CommonAssistData (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSCommonAssistData struct {
	_                              [0]struct{}                     `per:"extseq"`
	GNSSReferenceTime              *GNSSReferenceTime              `per:",optional"`
	GNSSReferenceLocation          *GNSSReferenceLocation          `per:",optional"`
	GNSSIonosphericModel           *GNSSIonosphericModel           `per:",optional"`
	GNSSEarthOrientationParameters *GNSSEarthOrientationParameters `per:",optional"`
}

// =====================================================================
// GNSS-ReferenceTimeReq (TS 37.355 §6.5.2.3)
// =====================================================================

type GNSSReferenceTimeReq struct {
	_                   [0]struct{} `per:"extseq"`
	GNSSTimeReqPrefList []GNSSID    `per:"SEQUENCE-OF,size:1..8"`
	GPSTOWAssistReq     *bool       `per:",optional"`
}

// =====================================================================
// GNSS-CommonAssistDataReq (TS 37.355 §6.5.2.3)
// =====================================================================

type GNSSCommonAssistDataReq struct {
	_                                 [0]struct{}           `per:"extseq"`
	GNSSReferenceTimeReq              *GNSSReferenceTimeReq `per:",optional"`
	GNSSReferenceLocationReq          *bool                 `per:",optional"`
	GNSSIonosphericModelReq           *bool                 `per:",optional"`
	GNSSEarthOrientationParametersReq *bool                 `per:",optional"`
}

// =====================================================================
// GNSS-SystemTime (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSSystemTime struct {
	_                        [0]struct{} `per:"extseq"`
	GNSSTimeID               GNSSID
	GNSSDayNumber            int64         `per:",range:0..32767"`
	GNSSTimeOfDay            int64         `per:",range:0..86399"`
	GNSSTimeOfDayFracMsec    *int64        `per:",optional,range:0..999"`
	NotificationOfLeapSecond *[]bool       `per:",optional,size:2"`
	GPSTOWAssist             *GPSTOWAssist `per:",optional"`
}

type GPSTOWAssist struct {
	_          [0]struct{} `per:"extseq"`
	GPSTOWMsec int64       `per:",range:0..604799"`
	GPSTOWFrac *int64      `per:",optional,range:0..999"`
}

// =====================================================================
// NetworkTime (TS 37.355 §6.5.2.2)
// =====================================================================

type NetworkTime struct {
	_                                        [0]struct{} `per:"extseq"`
	SecondsFromFrameStructureStart           int64       `per:",range:0..12533"`
	FractionalSecondsFromFrameStructureStart int64       `per:",range:0..3999999"`
	FrameStructureStart                      *int64      `per:",optional,range:0..7"`
}

// =====================================================================
// GNSS-GenericAssistData (TS 37.355 §6.5.2.2)
// =====================================================================

type GNSSGenericAssistData struct {
	List []GNSSGenericAssistDataElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSGenericAssistDataElement struct {
	_                           [0]struct{} `per:"extseq"`
	GNSSID                      GNSSID
	SBASID                      *SBASID   `per:",optional"`
	GNSSTimeModels              *per.Null `per:",optional"`
	GNSSDifferentialCorrections *per.Null `per:",optional"`
	GNSSNavigationModel         *per.Null `per:",optional"`
	GNSSRealTimeIntegrity       *per.Null `per:",optional"`
	GNSSDataBitAssistance       *per.Null `per:",optional"`
	GNSSAcquisitionAssistance   *per.Null `per:",optional"`
	GNSSAlmanac                 *per.Null `per:",optional"`
	GNSSUTCModel                *per.Null `per:",optional"`
	GNSSAuxiliaryInformation    *per.Null `per:",optional"`
}

// =====================================================================
// GNSS-GenericAssistDataReq (TS 37.355 §6.5.2.3)
// =====================================================================

type GNSSGenericAssistDataReq struct {
	List []GNSSGenericAssistDataReqElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSGenericAssistDataReqElement struct {
	_                              [0]struct{} `per:"extseq"`
	GNSSID                         GNSSID
	SBASID                         *SBASID   `per:",optional"`
	GNSSTimeModelsReq              *per.Null `per:",optional"`
	GNSSDifferentialCorrectionsReq *per.Null `per:",optional"`
	GNSSNavigationModelReq         *per.Null `per:",optional"`
	GNSSRealTimeIntegrityReq       *bool     `per:",optional"`
	GNSSDataBitAssistanceReq       *bool     `per:",optional"`
	GNSSAcquisitionAssistanceReq   *bool     `per:",optional"`
	GNSSAlmanacReq                 *bool     `per:",optional"`
	GNSSUTCModelReq                *bool     `per:",optional"`
	GNSSAuxiliaryInformationReq    *bool     `per:",optional"`
}

// =====================================================================
// GNSS-SignalMeasurementInformation (TS 37.355 §6.5.2.6)
// =====================================================================

type GNSSSignalMeasurementInformation struct {
	_                        [0]struct{} `per:"extseq"`
	MeasurementReferenceTime MeasurementReferenceTime
	GNSSMeasurementList      []GNSSMeasurementElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSMeasurementElement struct {
	_             [0]struct{} `per:"extseq"`
	GNSSSignal    GNSSSignalID
	GNSSCodePhase int64  `per:",range:0..65535"`
	GNSSDoppler   int64  `per:",range:-32767..32767"`
	GNSSCNo       int64  `per:",range:0..127"`
	GNSSADR       *int64 `per:",optional,range:-32767..32767"`
}

type GNSSSignalID struct {
	_          [0]struct{} `per:"extseq"`
	GNSSID     GNSSID
	GNSSSignal []bool `per:",size:8"`
}

// =====================================================================
// GNSS-SupportList (TS 37.355 §6.5.2.9)
// =====================================================================

type AssistanceDataSupportList struct {
	_                     [0]struct{} `per:"extseq"`
	GNSSReferenceTime     *bool       `per:",optional"`
	GNSSReferenceLocation *bool       `per:",optional"`
	GNSSIPSMODEL          *bool       `per:",optional"`
	GNSSEarthOrientation  *bool       `per:",optional"`
	GNSSTime              *bool       `per:",optional"`
	GNSSDifferential      *bool       `per:",optional"`
	GNSSNavigation        *bool       `per:",optional"`
	GNSSRealTimeIntegrity *bool       `per:",optional"`
	GNSSDataBit           *bool       `per:",optional"`
	GNSSAcquisition       *bool       `per:",optional"`
	GNSSAlmanac           *bool       `per:",optional"`
	GNSSUTCModel          *bool       `per:",optional"`
	GNSSAuxInfo           *bool       `per:",optional"`
}
