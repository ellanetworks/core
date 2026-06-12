// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/free5gc/aper"

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
	LocationInformationType LocationInformationType `aper:"valueExt"`
	TriggeredReporting      *struct{}               `aper:"optional"`
	PeriodicalReporting     *struct{}               `aper:"optional"`
	AdditionalInformation   *struct{}               `aper:"optional"`
	QoS                     *QoS                    `aper:"optional,valueExt"`
	Environment             *struct{}               `aper:"optional"`
	LocationCoordinateTypes *struct{}               `aper:"optional"`
	VelocityTypes           *struct{}               `aper:"optional"`
}

//	LocationInformationType ::= ENUMERATED {
//	    locationEstimateRequired, locationMeasurementsRequired,
//	    locationEstimatePreferred, locationMeasurementsPreferred, ...,
//	    locationEstimateAndMeasurementsRequired-r18
//	}
const (
	LocationInformationTypeLocationEstimateRequired      aper.Enumerated = 0
	LocationInformationTypeLocationMeasurementsRequired  aper.Enumerated = 1
	LocationInformationTypeLocationEstimatePreferred     aper.Enumerated = 2
	LocationInformationTypeLocationMeasurementsPreferred aper.Enumerated = 3
)

type LocationInformationType struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:3,valueExt"`
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
	HorizontalAccuracy        *HorizontalAccuracy `aper:"optional"`
	VerticalCoordinateRequest bool
	VerticalAccuracy          *VerticalAccuracy `aper:"optional"`
	ResponseTime              *ResponseTime     `aper:"optional"`
	VelocityRequest           bool
}

// HorizontalAccuracy ::= SEQUENCE { accuracy INTEGER(0..127), confidence INTEGER(0..100), ... }
type HorizontalAccuracy struct {
	Accuracy   int64 `aper:"valueLB:0,valueUB:127"`
	Confidence int64 `aper:"valueLB:0,valueUB:100"`
}

// VerticalAccuracy ::= SEQUENCE { accuracy INTEGER(0..127), confidence INTEGER(0..100), ... }
type VerticalAccuracy struct {
	Accuracy   int64 `aper:"valueLB:0,valueUB:127"`
	Confidence int64 `aper:"valueLB:0,valueUB:100"`
}

// ResponseTime ::= SEQUENCE { time INTEGER (1..128), ..., [[ ... ]] }
type ResponseTime struct {
	Time int64 `aper:"valueLB:1,valueUB:128"`
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
	LocationEstimate *LocationCoordinates `aper:"optional,valueExt,valueLB:0,valueUB:6"`
	VelocityEstimate *Velocity            `aper:"optional,valueExt,valueLB:0,valueUB:3"`
	LocationError    *LocationError       `aper:"optional,valueExt"`
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
// Extensible CHOICE with 7 root alternatives → valueLB:0, valueUB:6, valueExt.
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
	Present                                           int
	EllipsoidPoint                                    *EllipsoidPoint
	EllipsoidPointWithUncertaintyCircle               *EllipsoidPointWithUncertaintyCircle
	EllipsoidPointWithUncertaintyEllipse              *EllipsoidPointWithUncertaintyEllipse
	Polygon                                           *Polygon
	EllipsoidPointWithAltitude                        *EllipsoidPointWithAltitude
	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid *EllipsoidPointWithAltitudeAndUncertaintyEllipsoid
	EllipsoidArc                                      *EllipsoidArc
}

// =====================================================================
// Geographic Shapes (TS 23.032 / TS 37.355 §6.4.1)
// =====================================================================

//	Ellipsoid-Point ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607)
//	}
//
// NOTE: degreesLongitude uses valueLB:0,valueUB:16777215 (unsigned offset)
// instead of the spec's valueLB:-8388608,valueUB:8388607 (signed). This works
// around a bug in github.com/free5gc/aper v1.1.1 where the byte-length
// determinant for constrained INTEGERs with range > 65536 is computed from
// the original value instead of the offset (value - lowerBound). The APER
// bits on the wire are identical because the offset is the same; only the
// Go-side encoding/decode functions adjust by adding/subtracting 8388608.
const (
	EllipsoidPointLatitudeSignNorth aper.Enumerated = 0
	EllipsoidPointLatitudeSignSouth aper.Enumerated = 1
)

type EllipsoidPoint struct {
	LatitudeSign     aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude  int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude int64           `aper:"valueLB:0,valueUB:16777215"`
}

//	Ellipsoid-PointWithUncertaintyCircle ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    uncertainty     INTEGER (0..127)
//	}
type EllipsoidPointWithUncertaintyCircle struct {
	LatitudeSign     aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude  int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude int64           `aper:"valueLB:0,valueUB:16777215"`
	Uncertainty      int64           `aper:"valueLB:0,valueUB:127"`
}

//	EllipsoidPointWithAltitude ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    altitudeDirection   ENUMERATED {height, depth},
//	    altitude     INTEGER (0..32767)
//	}
const (
	EllipsoidPointWithAltitudeAltitudeDirectionHeight aper.Enumerated = 0
	EllipsoidPointWithAltitudeAltitudeDirectionDepth  aper.Enumerated = 1
)

type EllipsoidPointWithAltitude struct {
	LatitudeSign      aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude   int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude  int64           `aper:"valueLB:0,valueUB:16777215"`
	AltitudeDirection aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	Altitude          int64           `aper:"valueLB:0,valueUB:32767"`
}

//	EllipsoidPointWithUncertaintyEllipse ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    uncertaintySemiMajor  INTEGER (0..127),
//	    uncertaintySemiMinor  INTEGER (0..127),
//	    orientationMajorAxis  INTEGER (0..179),
//	    confidence     INTEGER (0..100)
//	}
type EllipsoidPointWithUncertaintyEllipse struct {
	LatitudeSign         aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude      int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude     int64           `aper:"valueLB:0,valueUB:16777215"`
	UncertaintySemiMajor int64           `aper:"valueLB:0,valueUB:127"`
	UncertaintySemiMinor int64           `aper:"valueLB:0,valueUB:127"`
	OrientationMajorAxis int64           `aper:"valueLB:0,valueUB:179"`
	Confidence           int64           `aper:"valueLB:0,valueUB:100"`
}

//	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    altitudeDirection, altitude,
//	    uncertaintySemiMajor, uncertaintySemiMinor, orientationMajorAxis,
//	    uncertaintyAltitude, confidence
//	}
type EllipsoidPointWithAltitudeAndUncertaintyEllipsoid struct {
	LatitudeSign         aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude      int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude     int64           `aper:"valueLB:0,valueUB:16777215"`
	AltitudeDirection    aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	Altitude             int64           `aper:"valueLB:0,valueUB:32767"`
	UncertaintySemiMajor int64           `aper:"valueLB:0,valueUB:127"`
	UncertaintySemiMinor int64           `aper:"valueLB:0,valueUB:127"`
	OrientationMajorAxis int64           `aper:"valueLB:0,valueUB:179"`
	UncertaintyAltitude  int64           `aper:"valueLB:0,valueUB:127"`
	Confidence           int64           `aper:"valueLB:0,valueUB:100"`
}

//	EllipsoidArc ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    innerRadius, uncertaintyRadius, offsetAngle, includedAngle, confidence
//	}
type EllipsoidArc struct {
	LatitudeSign      aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude   int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude  int64           `aper:"valueLB:0,valueUB:16777215"`
	InnerRadius       int64           `aper:"valueLB:0,valueUB:65535"`
	UncertaintyRadius int64           `aper:"valueLB:0,valueUB:127"`
	OffsetAngle       int64           `aper:"valueLB:0,valueUB:179"`
	IncludedAngle     int64           `aper:"valueLB:0,valueUB:179"`
	Confidence        int64           `aper:"valueLB:0,valueUB:100"`
}

// Polygon ::= SEQUENCE (SIZE (3..15)) OF PolygonPoints
type Polygon struct {
	List []PolygonPoint `aper:"sizeLB:3,sizeUB:15"`
}

type PolygonPoint struct {
	LatitudeSign     aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	DegreesLatitude  int64           `aper:"valueLB:0,valueUB:8388607"`
	DegreesLongitude int64           `aper:"valueLB:0,valueUB:16777215"`
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
	Present                                      int
	HorizontalVelocity                           *HorizontalVelocity
	HorizontalWithVerticalVelocity               *HorizontalWithVerticalVelocity
	HorizontalVelocityWithUncertainty            *HorizontalVelocityWithUncertainty
	HorizontalWithVerticalVelocityAndUncertainty *HorizontalWithVerticalVelocityAndUncertainty
}

type HorizontalVelocity struct {
	Bearing         int64 `aper:"valueLB:0,valueUB:359"`
	HorizontalSpeed int64 `aper:"valueLB:0,valueUB:2047"`
}

type HorizontalWithVerticalVelocity struct {
	Bearing           int64           `aper:"valueLB:0,valueUB:359"`
	HorizontalSpeed   int64           `aper:"valueLB:0,valueUB:2047"`
	VerticalDirection aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	VerticalSpeed     int64           `aper:"valueLB:0,valueUB:255"`
}

type HorizontalVelocityWithUncertainty struct {
	Bearing          int64 `aper:"valueLB:0,valueUB:359"`
	HorizontalSpeed  int64 `aper:"valueLB:0,valueUB:2047"`
	UncertaintySpeed int64 `aper:"valueLB:0,valueUB:255"`
}

type HorizontalWithVerticalVelocityAndUncertainty struct {
	Bearing                    int64           `aper:"valueLB:0,valueUB:359"`
	HorizontalSpeed            int64           `aper:"valueLB:0,valueUB:2047"`
	VerticalDirection          aper.Enumerated `aper:"valueLB:0,valueUB:1"`
	VerticalSpeed              int64           `aper:"valueLB:0,valueUB:255"`
	HorizontalUncertaintySpeed int64           `aper:"valueLB:0,valueUB:255"`
	VerticalUncertaintySpeed   int64           `aper:"valueLB:0,valueUB:255"`
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
	LocationFailureCausePresentUndefined                                aper.Enumerated = 0
	LocationFailureCausePresentRequestedMethodNotSupported              aper.Enumerated = 1
	LocationFailureCausePresentPositionMethodFailure                    aper.Enumerated = 2
	LocationFailureCausePresentPeriodicLocationMeasurementsNotAvailable aper.Enumerated = 3
)

type LocationError struct {
	LocationFailureCause struct {
		Value aper.Enumerated `aper:"valueLB:0,valueUB:3,valueExt"`
	}
}
