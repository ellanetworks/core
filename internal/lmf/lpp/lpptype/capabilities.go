// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

type CommonIEsRequestCapabilities struct {
	_ [0]struct{} `per:"extseq"`
}

type AGNSSRequestCapabilities struct {
	_                            [0]struct{} `per:"extseq"`
	GNSSSupportListReq           bool
	AssistanceDataSupportListReq bool
	LocationVelocityTypesReq     bool
}

type OTDOARequestCapabilities struct {
	_ [0]struct{} `per:"extseq"`
}

type ECIDRequestCapabilities struct {
	_ [0]struct{} `per:"extseq"`
}

type CommonIEsProvideCapabilities struct {
	_ [0]struct{} `per:"extseq"`
}

type AGNSSProvideCapabilities struct {
	_                         [0]struct{}                `per:"extseq"`
	GNSSSupportList           *GNSSSupportList           `per:",optional"`
	AssistanceDataSupportList *AssistanceDataSupportList `per:",optional"`
	LocationCoordinateTypes   *LocationCoordinateTypes   `per:",optional"`
	VelocityTypes             *VelocityTypes             `per:",optional"`
}

type GNSSSupportList struct {
	List []GNSSSupportElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSSupportElement struct {
	_                          [0]struct{} `per:"extseq"`
	GNSSID                     GNSSID
	SBASIDs                    *SBASIDs `per:",optional"`
	AGNSSModes                 PositioningModes
	GNSSSignals                GNSSSignalIDs
	FTAMeasSupport             *FTAMeasSupport `per:",optional"`
	ADRSupport                 bool
	VelocityMeasurementSupport bool
}

type FTAMeasSupport struct {
	_        [0]struct{} `per:"extseq"`
	CellTime AccessTypes
	Mode     PositioningModes
}

type AssistanceDataSupportList struct {
	_                                [0]struct{} `per:"extseq"`
	GNSSCommonAssistanceDataSupport  GNSSCommonAssistanceDataSupport
	GNSSGenericAssistanceDataSupport GNSSGenericAssistanceDataSupport
}

type GNSSCommonAssistanceDataSupport struct {
	_                                     [0]struct{}                            `per:"extseq"`
	GNSSReferenceTimeSupport              *GNSSReferenceTimeSupport              `per:",optional"`
	GNSSReferenceLocationSupport          *GNSSReferenceLocationSupport          `per:",optional"`
	GNSSIonosphericModelSupport           *GNSSIonosphericModelSupport           `per:",optional"`
	GNSSEarthOrientationParametersSupport *GNSSEarthOrientationParametersSupport `per:",optional"`
}

type GNSSReferenceTimeSupport struct {
	_              [0]struct{} `per:"extseq"`
	GNSSSystemTime GNSSIDBitmap
	FTASupport     *AccessTypes `per:",optional"`
}

type GNSSReferenceLocationSupport struct {
	_ [0]struct{} `per:"extseq"`
}

const (
	IonoModelKlobuchar  = 0
	IonoModelNeQuick    = 1
	IonoModelKlobuchar2 = 2
)

type GNSSIonosphericModelSupport struct {
	_         [0]struct{} `per:"extseq"`
	IonoModel []bool      `per:",size:1..8"`
}

type GNSSEarthOrientationParametersSupport struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSGenericAssistanceDataSupport struct {
	List []GNSSGenericAssistDataSupportElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSGenericAssistDataSupportElement struct {
	_                                  [0]struct{} `per:"extseq"`
	GNSSID                             GNSSID
	SBASID                             *SBASID                             `per:",optional"`
	GNSSTimeModelsSupport              *GNSSTimeModelListSupport           `per:",optional"`
	GNSSDifferentialCorrectionsSupport *GNSSDifferentialCorrectionsSupport `per:",optional"`
	GNSSNavigationModelSupport         *GNSSNavigationModelSupport         `per:",optional"`
	GNSSRealTimeIntegritySupport       *GNSSRealTimeIntegritySupport       `per:",optional"`
	GNSSDataBitAssistanceSupport       *GNSSDataBitAssistanceSupport       `per:",optional"`
	GNSSAcquisitionAssistanceSupport   *GNSSAcquisitionAssistanceSupport   `per:",optional"`
	GNSSAlmanacSupport                 *GNSSAlmanacSupport                 `per:",optional"`
	GNSSUTCModelSupport                *GNSSUTCModelSupport                `per:",optional"`
	GNSSAuxiliaryInformationSupport    *GNSSAuxiliaryInformationSupport    `per:",optional"`
}

type GNSSTimeModelListSupport struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSDifferentialCorrectionsSupport struct {
	_                    [0]struct{} `per:"extseq"`
	GNSSSignalIDs        GNSSSignalIDs
	DGNSSValidityTimeSup bool
}

type GNSSNavigationModelSupport struct {
	_          [0]struct{} `per:"extseq"`
	ClockModel []bool      `per:",optional,size:1..8"`
	OrbitModel []bool      `per:",optional,size:1..8"`
}

type GNSSRealTimeIntegritySupport struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSDataBitAssistanceSupport struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSAcquisitionAssistanceSupport struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSAlmanacSupport struct {
	_            [0]struct{} `per:"extseq"`
	AlmanacModel []bool      `per:",optional,size:1..8"`
}

type GNSSUTCModelSupport struct {
	_        [0]struct{} `per:"extseq"`
	UTCModel []bool      `per:",optional,size:1..8"`
}

type GNSSAuxiliaryInformationSupport struct {
	_ [0]struct{} `per:"extseq"`
}

type LocationCoordinateTypes struct {
	_                                                 [0]struct{} `per:"extseq"`
	EllipsoidPoint                                    bool
	EllipsoidPointWithUncertaintyCircle               bool
	EllipsoidPointWithUncertaintyEllipse              bool
	Polygon                                           bool
	EllipsoidPointWithAltitude                        bool
	EllipsoidPointWithAltitudeAndUncertaintyEllipsoid bool
	EllipsoidArc                                      bool
}

type VelocityTypes struct {
	_                                            [0]struct{} `per:"extseq"`
	HorizontalVelocity                           bool
	HorizontalWithVerticalVelocity               bool
	HorizontalVelocityWithUncertainty            bool
	HorizontalWithVerticalVelocityAndUncertainty bool
}

const (
	OTDOAModeUEAssisted      = 0
	OTDOAModeUEAssistedNB    = 1
	OTDOAModeUEAssistedNBTDD = 2
)

type OTDOAProvideCapabilities struct {
	_         [0]struct{} `per:"extseq"`
	OTDOAMode []bool      `per:",size:1..8"`
}

const (
	ECIDMeasSupportedRSRP   = 0
	ECIDMeasSupportedRSRQ   = 1
	ECIDMeasSupportedUERxTx = 2
	ECIDMeasSupportedNRSRP  = 3
	ECIDMeasSupportedNRSRQ  = 4
)

type ECIDProvideCapabilities struct {
	_                 [0]struct{} `per:"extseq"`
	ECIDMeasSupported []bool      `per:",size:1..8"`
}
