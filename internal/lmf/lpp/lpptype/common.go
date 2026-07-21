// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/internal/per"

// =====================================================================
// CommonIEsRequestLocationInformation (TS 37.355 §6.4.2)
// =====================================================================

//	CommonIEsRequestLocationInformation ::= SEQUENCE {
//	    locationInformationType  LocationInformationType,
//	    triggeredReporting   TriggeredReportingCriteria OPTIONAL,
//	    periodicalReporting   PeriodicalReportingCriteria OPTIONAL,
//	    additionalInformation  AdditionalInformation  OPTIONAL,
//	    qos       QoS       OPTIONAL,
//	    environment     Environment     OPTIONAL,
//	    locationCoordinateTypes  LocationCoordinateTypes  OPTIONAL,
//	    velocityTypes    VelocityTypes    OPTIONAL,
//	    ...,
//	    [[ ... ]]
//	}
type CommonIEsRequestLocationInformation struct {
	_                       [0]struct{} `per:"extseq"`
	LocationInformationType LocationInformationType
	TriggeredReporting      *TriggeredReportingCriteria  `per:",optional"`
	PeriodicalReporting     *PeriodicalReportingCriteria `per:",optional"`
	AdditionalInformation   *AdditionalInformation       `per:",optional"`
	QoS                     *QoS                         `per:",optional"`
	Environment             *Environment                 `per:",optional"`
	LocationCoordinateTypes *LocationCoordinateTypes     `per:",optional"`
	VelocityTypes           *VelocityTypes               `per:",optional"`
}

//	LocationInformationType ::= ENUMERATED {
//	    locationEstimateRequired, locationMeasurementsRequired,
//	    locationEstimatePreferred, locationMeasurementsPreferred, ...,
//	    locationEstimateAndMeasurementsRequired-r18
//	}
const (
	LocationInformationTypeLocationEstimateRequired      int64 = 0
	LocationInformationTypeLocationMeasurementsRequired  int64 = 1
	LocationInformationTypeLocationEstimatePreferred     int64 = 2
	LocationInformationTypeLocationMeasurementsPreferred int64 = 3
)

type LocationInformationType struct {
	Value int64 `per:",range:0..3,..."`
}

//	QoS ::= SEQUENCE {
//	    horizontalAccuracy   HorizontalAccuracy  OPTIONAL,
//	    verticalCoordinateRequest BOOLEAN,
//	    verticalAccuracy   VerticalAccuracy  OPTIONAL,
//	    responseTime    ResponseTime   OPTIONAL,
//	    velocityRequest    BOOLEAN,
//	    ...,
//	}
type QoS struct {
	_                         [0]struct{}         `per:"extseq"`
	HorizontalAccuracy        *HorizontalAccuracy `per:",optional"`
	VerticalCoordinateRequest bool
	VerticalAccuracy          *VerticalAccuracy `per:",optional"`
	ResponseTime              *ResponseTime     `per:",optional"`
	VelocityRequest           bool
}

// HorizontalAccuracy ::= SEQUENCE { accuracy INTEGER(0..127), confidence INTEGER(0..100), ... }
type HorizontalAccuracy struct {
	_          [0]struct{} `per:"extseq"`
	Accuracy   int64       `per:",range:0..127"`
	Confidence int64       `per:",range:0..100"`
}

// VerticalAccuracy ::= SEQUENCE { accuracy INTEGER(0..127), confidence INTEGER(0..100), ... }
type VerticalAccuracy struct {
	_          [0]struct{} `per:"extseq"`
	Accuracy   int64       `per:",range:0..127"`
	Confidence int64       `per:",range:0..100"`
}

// ResponseTime ::= SEQUENCE { time INTEGER (1..128), ..., [[ ... ]] }
type ResponseTime struct {
	_    [0]struct{} `per:"extseq"`
	Time int64       `per:",range:1..128"`
}

// =====================================================================
// CommonIEsProvideLocationInformation (TS 37.355 §6.4.2)
// =====================================================================

//	CommonIEsProvideLocationInformation ::= SEQUENCE {
//	    locationEstimate   LocationCoordinates  OPTIONAL,
//	    velocityEstimate   Velocity    OPTIONAL,
//	    locationError    LocationError   OPTIONAL,
//	    ...,
//	    [[ ... ]]
//	}
type CommonIEsProvideLocationInformation struct {
	_                [0]struct{}          `per:"extseq"`
	LocationEstimate *LocationCoordinates `per:",optional"`
	VelocityEstimate *Velocity            `per:",optional"`
	LocationError    *LocationError       `per:",optional"`
}

// =====================================================================
// LocationCoordinates (TS 37.355 §6.4.2)
// =====================================================================

//	LocationCoordinates ::= CHOICE {
//	    ellipsoidPoint        Ellipsoid-Point,
//	    ellipsoidPointWithUncertaintyCircle   Ellipsoid-PointWithUncertaintyCircle,
//	    ellipsoidPointWithUncertaintyEllipse  EllipsoidPointWithUncertaintyEllipse,
//	    polygon          Polygon,
//	    ellipsoidPointWithAltitude     EllipsoidPointWithAltitude,
//	    ellipsoidPointWithAltitudeAndUncertaintyEllipsoid ...,
//	    ellipsoidArc        EllipsoidArc,
//	    ...,
//	    [[ ... ]]
//	}
//
// Extensible CHOICE with 7 root alternatives.
const (
	LocationCoordinatesPresentNothing int = iota
	LocationCoordinatesPresentEllipsoidPoint
	LocationCoordinatesPresentEllipsoidPointWithUncertaintyCircle
	LocationCoordinatesPresentEllipsoidPointWithUncertaintyEllipse
	LocationCoordinatesPresentPolygon
	LocationCoordinatesPresentEllipsoidPointWithAltitude
	LocationCoordinatesPresentEllipsoidPointWithAltitudeAndUncertaintyEllipsoid
	LocationCoordinatesPresentEllipsoidArc
)

type LocationCoordinates struct {
	_                                                 [0]struct{}                                        `per:"extseq"`
	EllipsoidPoint                                    *EllipsoidPoint                                    `per:",choice:0,optional"`
	EllipsoidPointWithUncertaintyCircle               *EllipsoidPointWithUncertaintyCircle               `per:",choice:1,optional"`
	EllipsoidPointWithUncertaintyEllipse              *EllipsoidPointWithUncertaintyEllipse              `per:",choice:2,optional"`
	Polygon                                           *Polygon                                           `per:",choice:3,optional"`
	EllipsoidPointWithAltitude                        *EllipsoidPointWithAltitude                        `per:",choice:4,optional"`
	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid *EllipsoidPointWithAltitudeAndUncertaintyEllipsoid `per:",choice:5,optional"`
	EllipsoidArc                                      *EllipsoidArc                                      `per:",choice:6,optional"`
}

// =====================================================================
// Geographic Shapes (TS 23.032 / TS 37.355 §6.4.1)
// =====================================================================

//	Ellipsoid-Point ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607)
//	}
const (
	EllipsoidPointLatitudeSignNorth int64 = 0
	EllipsoidPointLatitudeSignSouth int64 = 1
)

type EllipsoidPoint struct {
	LatitudeSign     int64 `per:",range:0..1"`
	DegreesLatitude  int64 `per:",range:0..8388607"`
	DegreesLongitude int64 `per:",range:0..16777215"`
}

//	Ellipsoid-PointWithUncertaintyCircle ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    uncertainty     INTEGER (0..127)
//	}
type EllipsoidPointWithUncertaintyCircle struct {
	LatitudeSign     int64 `per:",range:0..1"`
	DegreesLatitude  int64 `per:",range:0..8388607"`
	DegreesLongitude int64 `per:",range:0..16777215"`
	Uncertainty      int64 `per:",range:0..127"`
}

//	EllipsoidPointWithAltitude ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    altitudeDirection   ENUMERATED {height, depth},
//	    altitude     INTEGER (0..32767)
//	}
const (
	EllipsoidPointWithAltitudeAltitudeDirectionHeight int64 = 0
	EllipsoidPointWithAltitudeAltitudeDirectionDepth  int64 = 1
)

type EllipsoidPointWithAltitude struct {
	LatitudeSign      int64 `per:",range:0..1"`
	DegreesLatitude   int64 `per:",range:0..8388607"`
	DegreesLongitude  int64 `per:",range:0..16777215"`
	AltitudeDirection int64 `per:",range:0..1"`
	Altitude          int64 `per:",range:0..32767"`
}

//	EllipsoidPointWithUncertaintyEllipse ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    uncertaintySemiMajor, uncertaintySemiMinor, orientationMajorAxis, confidence
//	}
type EllipsoidPointWithUncertaintyEllipse struct {
	LatitudeSign         int64 `per:",range:0..1"`
	DegreesLatitude      int64 `per:",range:0..8388607"`
	DegreesLongitude     int64 `per:",range:0..16777215"`
	UncertaintySemiMajor int64 `per:",range:0..127"`
	UncertaintySemiMinor int64 `per:",range:0..127"`
	OrientationMajorAxis int64 `per:",range:0..179"`
	Confidence           int64 `per:",range:0..100"`
}

//	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    altitudeDirection, altitude,
//	    uncertaintySemiMajor, uncertaintySemiMinor, orientationMajorAxis,
//	    uncertaintyAltitude, confidence
//	}
type EllipsoidPointWithAltitudeAndUncertaintyEllipsoid struct {
	LatitudeSign         int64 `per:",range:0..1"`
	DegreesLatitude      int64 `per:",range:0..8388607"`
	DegreesLongitude     int64 `per:",range:0..16777215"`
	AltitudeDirection    int64 `per:",range:0..1"`
	Altitude             int64 `per:",range:0..32767"`
	UncertaintySemiMajor int64 `per:",range:0..127"`
	UncertaintySemiMinor int64 `per:",range:0..127"`
	OrientationMajorAxis int64 `per:",range:0..179"`
	UncertaintyAltitude  int64 `per:",range:0..127"`
	Confidence           int64 `per:",range:0..100"`
}

//	EllipsoidArc ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    innerRadius, uncertaintyRadius, offsetAngle, includedAngle, confidence
//	}
type EllipsoidArc struct {
	LatitudeSign      int64 `per:",range:0..1"`
	DegreesLatitude   int64 `per:",range:0..8388607"`
	DegreesLongitude  int64 `per:",range:0..16777215"`
	InnerRadius       int64 `per:",range:0..65535"`
	UncertaintyRadius int64 `per:",range:0..127"`
	OffsetAngle       int64 `per:",range:0..179"`
	IncludedAngle     int64 `per:",range:0..179"`
	Confidence        int64 `per:",range:0..100"`
}

// Polygon ::= SEQUENCE (SIZE (3..15)) OF PolygonPoints
type Polygon struct {
	List []PolygonPoint `per:"SEQUENCE-OF,size:3..15"`
}

type PolygonPoint struct {
	LatitudeSign     int64 `per:",range:0..1"`
	DegreesLatitude  int64 `per:",range:0..8388607"`
	DegreesLongitude int64 `per:",range:0..16777215"`
}

// =====================================================================
// Velocity (TS 37.355 §6.4.1 / TS 23.032)
// =====================================================================

//	Velocity ::= CHOICE {
//	    horizontalVelocity       HorizontalVelocity,
//	    horizontalWithVerticalVelocity    HorizontalWithVerticalVelocity,
//	    horizontalVelocityWithUncertainty   HorizontalVelocityWithUncertainty,
//	    horizontalWithVerticalVelocityAndUncertainty ...,
//	    ...
//	}
const (
	VelocityPresentNothing int = iota
	VelocityPresentHorizontalVelocity
	VelocityPresentHorizontalWithVerticalVelocity
	VelocityPresentHorizontalVelocityWithUncertainty
	VelocityPresentHorizontalWithVerticalVelocityAndUncertainty
)

type Velocity struct {
	_                                            [0]struct{}                                   `per:"extseq"`
	HorizontalVelocity                           *HorizontalVelocity                           `per:",choice:0,optional"`
	HorizontalWithVerticalVelocity               *HorizontalWithVerticalVelocity               `per:",choice:1,optional"`
	HorizontalVelocityWithUncertainty            *HorizontalVelocityWithUncertainty            `per:",choice:2,optional"`
	HorizontalWithVerticalVelocityAndUncertainty *HorizontalWithVerticalVelocityAndUncertainty `per:",choice:3,optional"`
}

type HorizontalVelocity struct {
	Bearing         int64 `per:",range:0..359"`
	HorizontalSpeed int64 `per:",range:0..2047"`
}

type HorizontalWithVerticalVelocity struct {
	Bearing           int64 `per:",range:0..359"`
	HorizontalSpeed   int64 `per:",range:0..2047"`
	VerticalDirection int64 `per:",range:0..1"`
	VerticalSpeed     int64 `per:",range:0..255"`
}

type HorizontalVelocityWithUncertainty struct {
	Bearing          int64 `per:",range:0..359"`
	HorizontalSpeed  int64 `per:",range:0..2047"`
	UncertaintySpeed int64 `per:",range:0..255"`
}

type HorizontalWithVerticalVelocityAndUncertainty struct {
	Bearing                    int64 `per:",range:0..359"`
	HorizontalSpeed            int64 `per:",range:0..2047"`
	VerticalDirection          int64 `per:",range:0..1"`
	VerticalSpeed              int64 `per:",range:0..255"`
	HorizontalUncertaintySpeed int64 `per:",range:0..255"`
	VerticalUncertaintySpeed   int64 `per:",range:0..255"`
}

// =====================================================================
// LocationError (TS 37.355 §6.4.2)
// =====================================================================

// LocationError ::= SEQUENCE { locationfailurecause LocationFailureCause, ... }
//
//	LocationFailureCause ::= ENUMERATED {
//	    undefined, requestedMethodNotSupported, positionMethodFailure,
//	    periodicLocationMeasurementsNotAvailable, ...
//	}
const (
	LocationFailureCausePresentUndefined                                int64 = 0
	LocationFailureCausePresentRequestedMethodNotSupported              int64 = 1
	LocationFailureCausePresentPositionMethodFailure                    int64 = 2
	LocationFailureCausePresentPeriodicLocationMeasurementsNotAvailable int64 = 3
)

type LocationError struct {
	_                    [0]struct{} `per:"extseq"`
	LocationFailureCause int64       `per:",range:0..3,..."`
}

// =====================================================================
// CommonIEsRequestAssistanceData (TS 37.355 §6.3)
// =====================================================================

//	CommonIEsRequestAssistanceData ::= SEQUENCE {
//	    primaryCellID ECGI OPTIONAL,
//	    ...,
//	    [[ segmentationInfo-r14 SegmentationInfo-r14 OPTIONAL ],
//	     [[ periodicAssistanceDataReq-r15 PeriodicAssistanceDataControlParameters-r15 OPTIONAL,
//	        primaryCellID-r15 NCGI-r15 OPTIONAL ]]]
//	}
type CommonIEsRequestAssistanceData struct {
	_                         [0]struct{}                                 `per:"extseq"`
	PrimaryCellID             *ECGI                                       `per:",optional"`
	SegmentationInfo          *SegmentationInfoR14                        `per:",optional"`
	PeriodicAssistanceDataReq *PeriodicAssistanceDataControlParametersR15 `per:",optional"`
	PrimaryCellIDR15          *per.Null                                   `per:",optional"`
}

// =====================================================================
// CommonIEsProvideAssistanceData (TS 37.355 §6.3)
// =====================================================================

//	CommonIEsProvideAssistanceData ::= SEQUENCE {
//	    ...,
//	    [[ segmentationInfo-r14 SegmentationInfo-r14 OPTIONAL ],
//	     [[ periodicAssistanceData-r15 PeriodicAssistanceDataControlParameters-r15 OPTIONAL ]]]
//	}
type CommonIEsProvideAssistanceData struct {
	_                      [0]struct{}                                 `per:"extseq"`
	SegmentationInfo       *SegmentationInfoR14                        `per:",optional"`
	PeriodicAssistanceData *PeriodicAssistanceDataControlParametersR15 `per:",optional"`
}

// =====================================================================
// CommonIEsRequestCapabilities (TS 37.355 §6.3)
// =====================================================================

type CommonIEsRequestCapabilities struct {
	_                            [0]struct{} `per:"extseq"`
	LppMessageSegmentationReqR14 *[]bool     `per:",optional,size:2"`
	RemoteUEIndicationReqR18     *int64      `per:",optional,range:0..0,..."`
}

// =====================================================================
// CommonIEsProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

type CommonIEsProvideCapabilities struct {
	_                                          [0]struct{}          `per:"extseq"`
	SegmentationInfoR14                        *SegmentationInfoR14 `per:",optional"`
	LppMessageSegmentationR14                  *[]bool              `per:",optional,size:2"`
	RemoteUEIndicationR18                      *bool                `per:",optional"`
	LocationEstimateAndMeasurementReportingR18 *int64               `per:",optional,range:0..0,..."`
}

// =====================================================================
// SegmentationInfo-r14 (TS 37.355 §6.3)
// =====================================================================

const (
	SegmentationInfoR14NoMoreMessages       int64 = 0
	SegmentationInfoR14MoreMessagesOnTheWay int64 = 1
)

type SegmentationInfoR14 struct {
	Value int64 `per:",range:0..1,..."`
}

// =====================================================================
// PeriodicAssistanceDataControlParameters-r15 (TS 37.355 §6.3)
// =====================================================================

type PeriodicAssistanceDataControlParametersR15 struct {
	PeriodicSessionIDR15 PeriodicSessionIDR15
}

type PeriodicSessionIDR15 int64

// =====================================================================
// AdditionalInformation (TS 37.355 §6.4.2)
// =====================================================================

const (
	AdditionalInformationOnlyReturnInformationRequested int64 = 0
	AdditionalInformationMayReturnAdditionalInformation int64 = 1
)

type AdditionalInformation struct {
	Value int64 `per:",range:0..1,..."`
}

// =====================================================================
// Environment (TS 37.355 §6.4.2)
// =====================================================================

const (
	EnvironmentBadArea    int64 = 0
	EnvironmentNotBadArea int64 = 1
)

type Environment struct {
	Value int64 `per:",range:0..1,..."`
}

// =====================================================================
// TriggeredReportingCriteria (TS 37.355 §6.4.2)
// =====================================================================

type TriggeredReportingCriteria struct {
	CellChange        bool
	ReportingDuration ReportingDuration
}

type ReportingDuration int64

// =====================================================================
// PeriodicalReportingCriteria (TS 37.355 §6.4.2)
// =====================================================================

type PeriodicalReportingCriteria struct {
	_                              [0]struct{}                     `per:"extseq"`
	ReportingAmount                *int64                          `per:",optional,range:0..6,..."`
	PeriodicalReportingCriteriaExt *PeriodicalReportingCriteriaExt `per:",optional"`
}

type PeriodicalReportingCriteriaExt struct {
	_                   [0]struct{} `per:"extseq"`
	ReportingIntervalMs *int64      `per:",optional,range:0..31"`
}

// =====================================================================
// VelocityTypes (TS 37.355 §6.4.1)
// =====================================================================

type VelocityTypes struct {
	_                              [0]struct{} `per:"extseq"`
	HorizontalVelocity             bool
	HorizontalWithVerticalVelocity *bool `per:",optional"`
}

// =====================================================================
// LocationCoordinateTypes (TS 37.355 §6.4.1)
// =====================================================================

type LocationCoordinateTypes struct {
	_                                                 [0]struct{} `per:"extseq"`
	EllipsoidPoint                                    bool
	EllipsoidPointWithUncertaintyCircle               *bool `per:",optional"`
	EllipsoidPointWithUncertaintyEllipse              *bool `per:",optional"`
	Polygon                                           *bool `per:",optional"`
	EllipsoidPointWithAltitude                        *bool `per:",optional"`
	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid *bool `per:",optional"`
	EllipsoidArc                                      *bool `per:",optional"`
}

// =====================================================================
// OTDOARequestCapabilities (TS 37.355 §6.3)
// =====================================================================

type OTDOARequestCapabilities struct {
	_                          [0]struct{} `per:"extseq"`
	MaxFreqLayers              *int64      `per:",optional,range:1..2"`
	MaxNoOfEUTRANeighbourCells *int64      `per:",optional,range:1..24"`
	MultipathRSTD              *int64      `per:",optional,range:0..0,..."`
	MotionMeasurements         *int64      `per:",optional,range:0..0,..."`
}

// =====================================================================
// OTDOAProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

type OTDOAProvideCapabilities struct {
	_                          [0]struct{} `per:"extseq"`
	MaxFreqLayers              *int64      `per:",optional,range:1..2"`
	MaxNoOfEUTRANeighbourCells *int64      `per:",optional,range:1..24"`
	AdditionalPRSconfigMaxR14  *int64      `per:",optional,range:1..8,..."`
	MotionTimeSource           *int64      `per:",optional,range:0..0,..."`
}

// =====================================================================
// ECIDRequestCapabilities (TS 37.355 §6.3)
// =====================================================================

type ECIDRequestCapabilities struct {
	_                      [0]struct{} `per:"extseq"`
	RequestedMeasurements  *[]bool     `per:",optional,size:1..8"`
	NrsrpResultR14         *bool       `per:",optional"`
	NrsrqResultR14         *bool       `per:",optional"`
	CarrierFreqOffsetNBR14 *bool       `per:",optional"`
	NrsrpResultV1470       *bool       `per:",optional,range:0..0,..."`
	NrsrqResultV1470       *bool       `per:",optional,range:0..0,..."`
}

// =====================================================================
// ECIDProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

type ECIDProvideCapabilities struct {
	_                      [0]struct{} `per:"extseq"`
	RequestedMeasurements  *[]bool     `per:",optional,size:1..8"`
	NrsrpResultR14         *bool       `per:",optional"`
	NrsrqResultR14         *bool       `per:",optional"`
	CarrierFreqOffsetNBR14 *bool       `per:",optional"`
	NrsrpResultV1470       *bool       `per:",optional,range:0..0,..."`
	NrsrqResultV1470       *bool       `per:",optional,range:0..0,..."`
}

// =====================================================================
// GNSS-LocationServerErrorCauses (TS 37.355 §6.5.2.12)
// =====================================================================

const (
	GNSSLocationServerErrorCausesUndefined                                       int64 = 0
	GNSSLocationServerErrorCausesAssistanceDataNotSupportedByServer              int64 = 1
	GNSSLocationServerErrorCausesAssistanceDataSupportedButCurrentlyNotAvailable int64 = 2
)

type GNSSLocationServerErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:",range:0..2,..."`
}

// =====================================================================
// GNSS-TargetDeviceErrorCauses (TS 37.355 §6.5.2.12)
// =====================================================================

const (
	GNSSTargetDeviceErrorCausesUndefined                  int64 = 0
	GNSSTargetDeviceErrorCausesGNSSMeasurementUnavailable int64 = 1
	GNSSTargetDeviceErrorCausesGNSSMeasurementInaccurate  int64 = 2
)

type GNSSTargetDeviceErrorCauses struct {
	_     [0]struct{} `per:"extseq"`
	Cause int64       `per:",range:0..2,..."`
}

// =====================================================================
// A-GNSS-Error (TS 37.355 §6.5.2.12)
// =====================================================================

type GNSSError struct {
	_                         [0]struct{}                    `per:"extseq"`
	LocationServerErrorCauses *GNSSLocationServerErrorCauses `per:",choice:0,optional"`
	TargetDeviceErrorCauses   *GNSSTargetDeviceErrorCauses   `per:",choice:1,optional"`
}

// =====================================================================
// SBAS-IDs (TS 37.355 §6.4.1)
// =====================================================================

type SBASIDs struct {
	_       [0]struct{} `per:"extseq"`
	SBASIDs []bool      `per:",size:1..4"`
}

type SBASID struct {
	_    [0]struct{} `per:"extseq"`
	SBAS int64       `per:",range:0..3,..."`
}

// =====================================================================
// AccessTypes (TS 37.355 §6.5.2.9)
// =====================================================================

type AccessTypes struct {
	_      [0]struct{} `per:"extseq"`
	Access []bool      `per:",size:1..8"`
}

// =====================================================================
// FTAMeasSupport (TS 37.355 §6.5.2.9)
// =====================================================================

type FTAMeasSupport struct {
	_        [0]struct{} `per:"extseq"`
	CellTime AccessTypes
	Mode     PositioningModes
}
