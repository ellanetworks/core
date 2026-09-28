// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

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
	_                       [0]struct{}                  `per:"extseq"`
	LocationInformationType LocationInformationType      `per:"ENUMERATED,range:0..3,...,extvalues:1"`
	TriggeredReporting      *TriggeredReportingCriteria  `per:",optional"`
	PeriodicalReporting     *PeriodicalReportingCriteria `per:",optional"`
	AdditionalInformation   *AdditionalInformation       `per:"ENUMERATED,optional,range:0..1,...,extvalues:0"`
	QoS                     *QoS                         `per:",optional"`
	Environment             *Environment                 `per:"ENUMERATED,optional,range:0..2,...,extvalues:0"`
	LocationCoordinateTypes *LocationCoordinateTypes     `per:",optional"`
	VelocityTypes           *VelocityTypes               `per:",optional"`
}

//	LocationInformationType ::= ENUMERATED {
//	    locationEstimateRequired, locationMeasurementsRequired,
//	    locationEstimatePreferred, locationMeasurementsPreferred, ...,
//	    locationEstimateAndMeasurementsRequired-r18
//	}
type LocationInformationType int64

const (
	LocationInformationTypeLocationEstimateRequired                LocationInformationType = 0
	LocationInformationTypeLocationMeasurementsRequired            LocationInformationType = 1
	LocationInformationTypeLocationEstimatePreferred               LocationInformationType = 2
	LocationInformationTypeLocationMeasurementsPreferred           LocationInformationType = 3
	LocationInformationTypeLocationEstimateAndMeasurementsRequired LocationInformationType = 4
)

type TriggeredReportingCriteria struct {
	_                 [0]struct{} `per:"extseq"`
	CellChange        bool
	ReportingDuration int64 `per:",range:0..255"`
}

type ReportingAmount int64

const (
	ReportingAmountRA1        ReportingAmount = 0
	ReportingAmountRA2        ReportingAmount = 1
	ReportingAmountRA4        ReportingAmount = 2
	ReportingAmountRA8        ReportingAmount = 3
	ReportingAmountRA16       ReportingAmount = 4
	ReportingAmountRA32       ReportingAmount = 5
	ReportingAmountRA64       ReportingAmount = 6
	ReportingAmountRAInfinity ReportingAmount = 7
)

type ReportingInterval int64

const (
	ReportingIntervalNoPeriodicalReporting ReportingInterval = 0
	ReportingIntervalRI025                 ReportingInterval = 1
	ReportingIntervalRI05                  ReportingInterval = 2
	ReportingIntervalRI1                   ReportingInterval = 3
	ReportingIntervalRI2                   ReportingInterval = 4
	ReportingIntervalRI4                   ReportingInterval = 5
	ReportingIntervalRI8                   ReportingInterval = 6
	ReportingIntervalRI16                  ReportingInterval = 7
	ReportingIntervalRI32                  ReportingInterval = 8
	ReportingIntervalRI64                  ReportingInterval = 9
)

type PeriodicalReportingCriteria struct {
	ReportingAmount   ReportingAmount   `per:"ENUMERATED,range:0..7,default:ReportingAmountRAInfinity"`
	ReportingInterval ReportingInterval `per:"ENUMERATED,range:0..9"`
}

func NewPeriodicalReportingCriteria(interval ReportingInterval) PeriodicalReportingCriteria {
	return PeriodicalReportingCriteria{ReportingAmount: ReportingAmountRAInfinity, ReportingInterval: interval}
}

type AdditionalInformation int64

const (
	AdditionalInformationOnlyReturnInformationRequested AdditionalInformation = 0
	AdditionalInformationMayReturnAdditionalInformation AdditionalInformation = 1
)

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

type Environment int64

const (
	EnvironmentBadArea    Environment = 0
	EnvironmentNotBadArea Environment = 1
	EnvironmentMixedArea  Environment = 2
)

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

type LatitudeSign int64

const (
	LatitudeSignNorth LatitudeSign = 0
	LatitudeSignSouth LatitudeSign = 1
)

type AltitudeDirection int64

const (
	AltitudeDirectionHeight AltitudeDirection = 0
	AltitudeDirectionDepth  AltitudeDirection = 1
)

//	Ellipsoid-Point ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607)
//	}
type EllipsoidPoint struct {
	LatitudeSign     LatitudeSign `per:"ENUMERATED,range:0..1"`
	DegreesLatitude  int64        `per:",range:0..8388607"`
	DegreesLongitude int64        `per:",range:-8388608..8388607"`
}

//	Ellipsoid-PointWithUncertaintyCircle ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    uncertainty     INTEGER (0..127)
//	}
type EllipsoidPointWithUncertaintyCircle struct {
	LatitudeSign     LatitudeSign `per:"ENUMERATED,range:0..1"`
	DegreesLatitude  int64        `per:",range:0..8388607"`
	DegreesLongitude int64        `per:",range:-8388608..8388607"`
	Uncertainty      int64        `per:",range:0..127"`
}

//	EllipsoidPointWithUncertaintyEllipse ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    uncertaintySemiMajor, uncertaintySemiMinor, orientationMajorAxis, confidence
//	}
type EllipsoidPointWithUncertaintyEllipse struct {
	LatitudeSign         LatitudeSign `per:"ENUMERATED,range:0..1"`
	DegreesLatitude      int64        `per:",range:0..8388607"`
	DegreesLongitude     int64        `per:",range:-8388608..8388607"`
	UncertaintySemiMajor int64        `per:",range:0..127"`
	UncertaintySemiMinor int64        `per:",range:0..127"`
	OrientationMajorAxis int64        `per:",range:0..179"`
	Confidence           int64        `per:",range:0..100"`
}

// Polygon ::= SEQUENCE (SIZE (3..15)) OF PolygonPoints
type Polygon struct {
	List []PolygonPoints `per:"SEQUENCE-OF,size:3..15"`
}

type PolygonPoints struct {
	LatitudeSign     LatitudeSign `per:"ENUMERATED,range:0..1"`
	DegreesLatitude  int64        `per:",range:0..8388607"`
	DegreesLongitude int64        `per:",range:-8388608..8388607"`
}

//	EllipsoidPointWithAltitude ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    altitudeDirection   ENUMERATED {height, depth},
//	    altitude     INTEGER (0..32767)
//	}
type EllipsoidPointWithAltitude struct {
	LatitudeSign      LatitudeSign      `per:"ENUMERATED,range:0..1"`
	DegreesLatitude   int64             `per:",range:0..8388607"`
	DegreesLongitude  int64             `per:",range:-8388608..8388607"`
	AltitudeDirection AltitudeDirection `per:"ENUMERATED,range:0..1"`
	Altitude          int64             `per:",range:0..32767"`
}

//	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    altitudeDirection, altitude,
//	    uncertaintySemiMajor, uncertaintySemiMinor, orientationMajorAxis,
//	    uncertaintyAltitude, confidence
//	}
type EllipsoidPointWithAltitudeAndUncertaintyEllipsoid struct {
	LatitudeSign         LatitudeSign      `per:"ENUMERATED,range:0..1"`
	DegreesLatitude      int64             `per:",range:0..8388607"`
	DegreesLongitude     int64             `per:",range:-8388608..8388607"`
	AltitudeDirection    AltitudeDirection `per:"ENUMERATED,range:0..1"`
	Altitude             int64             `per:",range:0..32767"`
	UncertaintySemiMajor int64             `per:",range:0..127"`
	UncertaintySemiMinor int64             `per:",range:0..127"`
	OrientationMajorAxis int64             `per:",range:0..179"`
	UncertaintyAltitude  int64             `per:",range:0..127"`
	Confidence           int64             `per:",range:0..100"`
}

//	EllipsoidArc ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    innerRadius, uncertaintyRadius, offsetAngle, includedAngle, confidence
//	}
type EllipsoidArc struct {
	LatitudeSign      LatitudeSign `per:"ENUMERATED,range:0..1"`
	DegreesLatitude   int64        `per:",range:0..8388607"`
	DegreesLongitude  int64        `per:",range:-8388608..8388607"`
	InnerRadius       int64        `per:",range:0..65535"`
	UncertaintyRadius int64        `per:",range:0..127"`
	OffsetAngle       int64        `per:",range:0..179"`
	IncludedAngle     int64        `per:",range:0..179"`
	Confidence        int64        `per:",range:0..100"`
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

type VerticalDirection int64

const (
	VerticalDirectionUpward   VerticalDirection = 0
	VerticalDirectionDownward VerticalDirection = 1
)

type HorizontalVelocity struct {
	Bearing         int64 `per:",range:0..359"`
	HorizontalSpeed int64 `per:",range:0..2047"`
}

type HorizontalWithVerticalVelocity struct {
	Bearing           int64             `per:",range:0..359"`
	HorizontalSpeed   int64             `per:",range:0..2047"`
	VerticalDirection VerticalDirection `per:"ENUMERATED,range:0..1"`
	VerticalSpeed     int64             `per:",range:0..255"`
}

type HorizontalVelocityWithUncertainty struct {
	Bearing          int64 `per:",range:0..359"`
	HorizontalSpeed  int64 `per:",range:0..2047"`
	UncertaintySpeed int64 `per:",range:0..255"`
}

type HorizontalWithVerticalVelocityAndUncertainty struct {
	Bearing                    int64             `per:",range:0..359"`
	HorizontalSpeed            int64             `per:",range:0..2047"`
	VerticalDirection          VerticalDirection `per:"ENUMERATED,range:0..1"`
	VerticalSpeed              int64             `per:",range:0..255"`
	HorizontalUncertaintySpeed int64             `per:",range:0..255"`
	VerticalUncertaintySpeed   int64             `per:",range:0..255"`
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
type LocationFailureCause int64

const (
	LocationFailureCauseUndefined                                LocationFailureCause = 0
	LocationFailureCauseRequestedMethodNotSupported              LocationFailureCause = 1
	LocationFailureCausePositionMethodFailure                    LocationFailureCause = 2
	LocationFailureCausePeriodicLocationMeasurementsNotAvailable LocationFailureCause = 3
)

type LocationError struct {
	_                    [0]struct{}          `per:"extseq"`
	LocationFailureCause LocationFailureCause `per:"ENUMERATED,range:0..3,...,extvalues:0"`
}
