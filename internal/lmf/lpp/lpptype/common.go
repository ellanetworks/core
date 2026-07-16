// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

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
	LocationInformationType LocationInformationType
	TriggeredReporting      *struct{}
	PeriodicalReporting     *struct{}
	AdditionalInformation   *struct{}
	QoS                     *QoS
	Environment             *struct{}
	LocationCoordinateTypes *struct{}
	VelocityTypes           *struct{}
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
	Value aper.Enumerated
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
	HorizontalAccuracy        *HorizontalAccuracy
	VerticalCoordinateRequest bool
	VerticalAccuracy          *VerticalAccuracy
	ResponseTime              *ResponseTime
	VelocityRequest           bool
}

// HorizontalAccuracy ::= SEQUENCE { accuracy INTEGER(0..127), confidence INTEGER(0..100), ... }
type HorizontalAccuracy struct {
	Accuracy   int64
	Confidence int64
}

// VerticalAccuracy ::= SEQUENCE { accuracy INTEGER(0..127), confidence INTEGER(0..100), ... }
type VerticalAccuracy struct {
	Accuracy   int64
	Confidence int64
}

// ResponseTime ::= SEQUENCE { time INTEGER (1..128), ..., [[ ... ]] }
type ResponseTime struct {
	Time int64
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
	LocationEstimate *LocationCoordinates
	VelocityEstimate *Velocity
	LocationError    *LocationError
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
// NOTE: the codec encodes degreesLongitude as an unsigned offset in 0..16777215
// rather than the spec's signed -8388608..8388607. Under PER a constrained
// INTEGER is encoded as value - lowerBound, so an unsigned field biased by
// 2^23 produces bit-for-bit the same wire encoding as the signed range; the
// encode/decode helpers add and subtract 8388608 to bridge the two.
const (
	EllipsoidPointLatitudeSignNorth aper.Enumerated = 0
	EllipsoidPointLatitudeSignSouth aper.Enumerated = 1
)

type EllipsoidPoint struct {
	LatitudeSign     aper.Enumerated
	DegreesLatitude  int64
	DegreesLongitude int64
}

//	Ellipsoid-PointWithUncertaintyCircle ::= SEQUENCE {
//	    latitudeSign    ENUMERATED {north, south},
//	    degreesLatitude    INTEGER (0..8388607),
//	    degreesLongitude   INTEGER (-8388608..8388607),
//	    uncertainty     INTEGER (0..127)
//	}
type EllipsoidPointWithUncertaintyCircle struct {
	LatitudeSign     aper.Enumerated
	DegreesLatitude  int64
	DegreesLongitude int64
	Uncertainty      int64
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
	LatitudeSign      aper.Enumerated
	DegreesLatitude   int64
	DegreesLongitude  int64
	AltitudeDirection aper.Enumerated
	Altitude          int64
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
	LatitudeSign         aper.Enumerated
	DegreesLatitude      int64
	DegreesLongitude     int64
	UncertaintySemiMajor int64
	UncertaintySemiMinor int64
	OrientationMajorAxis int64
	Confidence           int64
}

//	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    altitudeDirection, altitude,
//	    uncertaintySemiMajor, uncertaintySemiMinor, orientationMajorAxis,
//	    uncertaintyAltitude, confidence
//	}
type EllipsoidPointWithAltitudeAndUncertaintyEllipsoid struct {
	LatitudeSign         aper.Enumerated
	DegreesLatitude      int64
	DegreesLongitude     int64
	AltitudeDirection    aper.Enumerated
	Altitude             int64
	UncertaintySemiMajor int64
	UncertaintySemiMinor int64
	OrientationMajorAxis int64
	UncertaintyAltitude  int64
	Confidence           int64
}

//	EllipsoidArc ::= SEQUENCE {
//	    latitudeSign, degreesLatitude, degreesLongitude,
//	    innerRadius, uncertaintyRadius, offsetAngle, includedAngle, confidence
//	}
type EllipsoidArc struct {
	LatitudeSign      aper.Enumerated
	DegreesLatitude   int64
	DegreesLongitude  int64
	InnerRadius       int64
	UncertaintyRadius int64
	OffsetAngle       int64
	IncludedAngle     int64
	Confidence        int64
}

// Polygon ::= SEQUENCE (SIZE (3..15)) OF PolygonPoints
type Polygon struct {
	List []PolygonPoint
}

type PolygonPoint struct {
	LatitudeSign     aper.Enumerated
	DegreesLatitude  int64
	DegreesLongitude int64
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
	Bearing         int64
	HorizontalSpeed int64
}

type HorizontalWithVerticalVelocity struct {
	Bearing           int64
	HorizontalSpeed   int64
	VerticalDirection aper.Enumerated
	VerticalSpeed     int64
}

type HorizontalVelocityWithUncertainty struct {
	Bearing          int64
	HorizontalSpeed  int64
	UncertaintySpeed int64
}

type HorizontalWithVerticalVelocityAndUncertainty struct {
	Bearing                    int64
	HorizontalSpeed            int64
	VerticalDirection          aper.Enumerated
	VerticalSpeed              int64
	HorizontalUncertaintySpeed int64
	VerticalUncertaintySpeed   int64
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
		Value aper.Enumerated
	}
}

// LocationFailureCauseString names a locationError cause.
func LocationFailureCauseString(c aper.Enumerated) string {
	switch c {
	case LocationFailureCausePresentUndefined:
		return "undefined"
	case LocationFailureCausePresentRequestedMethodNotSupported:
		return "requestedMethodNotSupported"
	case LocationFailureCausePresentPositionMethodFailure:
		return "positionMethodFailure"
	case LocationFailureCausePresentPeriodicLocationMeasurementsNotAvailable:
		return "periodicLocationMeasurementsNotAvailable"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}
