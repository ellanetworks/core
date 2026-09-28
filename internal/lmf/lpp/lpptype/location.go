// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

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

const (
	LocationInformationTypeLocationEstimateRequired                int64 = 0
	LocationInformationTypeLocationMeasurementsRequired            int64 = 1
	LocationInformationTypeLocationEstimatePreferred               int64 = 2
	LocationInformationTypeLocationMeasurementsPreferred           int64 = 3
	LocationInformationTypeLocationEstimateAndMeasurementsRequired int64 = 4
)

type LocationInformationType struct {
	Value int64 `per:"ENUMERATED,range:0..3,..."`
}

type TriggeredReportingCriteria struct {
	_                 [0]struct{} `per:"extseq"`
	CellChange        bool
	ReportingDuration int64 `per:",range:0..255"`
}

const (
	ReportingAmountRA1        int64 = 0
	ReportingAmountRA2        int64 = 1
	ReportingAmountRA4        int64 = 2
	ReportingAmountRA8        int64 = 3
	ReportingAmountRA16       int64 = 4
	ReportingAmountRA32       int64 = 5
	ReportingAmountRA64       int64 = 6
	ReportingAmountRAInfinity int64 = 7
)

const (
	ReportingIntervalNoPeriodicalReporting int64 = 0
	ReportingIntervalRI0Dot25              int64 = 1
	ReportingIntervalRI0Dot5               int64 = 2
	ReportingIntervalRI1                   int64 = 3
	ReportingIntervalRI2                   int64 = 4
	ReportingIntervalRI4                   int64 = 5
	ReportingIntervalRI8                   int64 = 6
	ReportingIntervalRI16                  int64 = 7
	ReportingIntervalRI32                  int64 = 8
	ReportingIntervalRI64                  int64 = 9
)

type PeriodicalReportingCriteria struct {
	ReportingAmount   int64 `per:"ENUMERATED,range:0..7,default:ReportingAmountRAInfinity"`
	ReportingInterval int64 `per:"ENUMERATED,range:0..9"`
}

const (
	AdditionalInformationOnlyReturnInformationRequested int64 = 0
	AdditionalInformationMayReturnAdditionalInformation int64 = 1
)

type AdditionalInformation struct {
	Value int64 `per:"ENUMERATED,range:0..1,..."`
}

type QoS struct {
	_                         [0]struct{}         `per:"extseq"`
	HorizontalAccuracy        *HorizontalAccuracy `per:",optional"`
	VerticalCoordinateRequest bool
	VerticalAccuracy          *VerticalAccuracy `per:",optional"`
	ResponseTime              *ResponseTime     `per:",optional"`
	VelocityRequest           bool
}

type HorizontalAccuracy struct {
	_          [0]struct{} `per:"extseq"`
	Accuracy   int64       `per:",range:0..127"`
	Confidence int64       `per:",range:0..100"`
}

type VerticalAccuracy struct {
	_          [0]struct{} `per:"extseq"`
	Accuracy   int64       `per:",range:0..127"`
	Confidence int64       `per:",range:0..100"`
}

type ResponseTime struct {
	_    [0]struct{} `per:"extseq"`
	Time int64       `per:",range:1..128"`
}

const (
	EnvironmentBadArea    int64 = 0
	EnvironmentNotBadArea int64 = 1
	EnvironmentMixedArea  int64 = 2
)

type Environment struct {
	Value int64 `per:"ENUMERATED,range:0..2,..."`
}

type CommonIEsProvideLocationInformation struct {
	_                [0]struct{}          `per:"extseq"`
	LocationEstimate *LocationCoordinates `per:",optional"`
	VelocityEstimate *Velocity            `per:",optional"`
	LocationError    *LocationError       `per:",optional"`
}

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

const (
	LatitudeSignNorth int64 = 0
	LatitudeSignSouth int64 = 1
)

const (
	AltitudeDirectionHeight int64 = 0
	AltitudeDirectionDepth  int64 = 1
)

type EllipsoidPoint struct {
	LatitudeSign     int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude  int64 `per:",range:0..8388607"`
	DegreesLongitude int64 `per:",range:-8388608..8388607"`
}

type EllipsoidPointWithUncertaintyCircle struct {
	LatitudeSign     int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude  int64 `per:",range:0..8388607"`
	DegreesLongitude int64 `per:",range:-8388608..8388607"`
	Uncertainty      int64 `per:",range:0..127"`
}

type EllipsoidPointWithUncertaintyEllipse struct {
	LatitudeSign         int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude      int64 `per:",range:0..8388607"`
	DegreesLongitude     int64 `per:",range:-8388608..8388607"`
	UncertaintySemiMajor int64 `per:",range:0..127"`
	UncertaintySemiMinor int64 `per:",range:0..127"`
	OrientationMajorAxis int64 `per:",range:0..179"`
	Confidence           int64 `per:",range:0..100"`
}

type Polygon struct {
	List []PolygonPoints `per:"SEQUENCE-OF,size:3..15"`
}

type PolygonPoints struct {
	LatitudeSign     int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude  int64 `per:",range:0..8388607"`
	DegreesLongitude int64 `per:",range:-8388608..8388607"`
}

type EllipsoidPointWithAltitude struct {
	LatitudeSign      int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude   int64 `per:",range:0..8388607"`
	DegreesLongitude  int64 `per:",range:-8388608..8388607"`
	AltitudeDirection int64 `per:"ENUMERATED,range:0..1"`
	Altitude          int64 `per:",range:0..32767"`
}

type EllipsoidPointWithAltitudeAndUncertaintyEllipsoid struct {
	LatitudeSign         int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude      int64 `per:",range:0..8388607"`
	DegreesLongitude     int64 `per:",range:-8388608..8388607"`
	AltitudeDirection    int64 `per:"ENUMERATED,range:0..1"`
	Altitude             int64 `per:",range:0..32767"`
	UncertaintySemiMajor int64 `per:",range:0..127"`
	UncertaintySemiMinor int64 `per:",range:0..127"`
	OrientationMajorAxis int64 `per:",range:0..179"`
	UncertaintyAltitude  int64 `per:",range:0..127"`
	Confidence           int64 `per:",range:0..100"`
}

type EllipsoidArc struct {
	LatitudeSign      int64 `per:"ENUMERATED,range:0..1"`
	DegreesLatitude   int64 `per:",range:0..8388607"`
	DegreesLongitude  int64 `per:",range:-8388608..8388607"`
	InnerRadius       int64 `per:",range:0..65535"`
	UncertaintyRadius int64 `per:",range:0..127"`
	OffsetAngle       int64 `per:",range:0..179"`
	IncludedAngle     int64 `per:",range:0..179"`
	Confidence        int64 `per:",range:0..100"`
}

type Velocity struct {
	_                                            [0]struct{}                                   `per:"extseq"`
	HorizontalVelocity                           *HorizontalVelocity                           `per:",choice:0,optional"`
	HorizontalWithVerticalVelocity               *HorizontalWithVerticalVelocity               `per:",choice:1,optional"`
	HorizontalVelocityWithUncertainty            *HorizontalVelocityWithUncertainty            `per:",choice:2,optional"`
	HorizontalWithVerticalVelocityAndUncertainty *HorizontalWithVerticalVelocityAndUncertainty `per:",choice:3,optional"`
}

const (
	VerticalDirectionUpward   int64 = 0
	VerticalDirectionDownward int64 = 1
)

type HorizontalVelocity struct {
	Bearing         int64 `per:",range:0..359"`
	HorizontalSpeed int64 `per:",range:0..2047"`
}

type HorizontalWithVerticalVelocity struct {
	Bearing           int64 `per:",range:0..359"`
	HorizontalSpeed   int64 `per:",range:0..2047"`
	VerticalDirection int64 `per:"ENUMERATED,range:0..1"`
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
	VerticalDirection          int64 `per:"ENUMERATED,range:0..1"`
	VerticalSpeed              int64 `per:",range:0..255"`
	HorizontalUncertaintySpeed int64 `per:",range:0..255"`
	VerticalUncertaintySpeed   int64 `per:",range:0..255"`
}

const (
	LocationFailureCauseUndefined                                int64 = 0
	LocationFailureCauseRequestedMethodNotSupported              int64 = 1
	LocationFailureCausePositionMethodFailure                    int64 = 2
	LocationFailureCausePeriodicLocationMeasurementsNotAvailable int64 = 3
)

type LocationError struct {
	_                    [0]struct{} `per:"extseq"`
	LocationFailureCause int64       `per:"ENUMERATED,range:0..3,..."`
}
