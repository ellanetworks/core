// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

type CommonIEsProvideAssistanceData struct {
	_ [0]struct{} `per:"extseq"`
}

type AGNSSProvideAssistanceData struct {
	_                     [0]struct{}            `per:"extseq"`
	GNSSCommonAssistData  *GNSSCommonAssistData  `per:",optional"`
	GNSSGenericAssistData *GNSSGenericAssistData `per:",optional"`
	GNSSError             *AGNSSError            `per:",optional"`
}

type GNSSCommonAssistData struct {
	_                              [0]struct{}                     `per:"extseq"`
	GNSSReferenceTime              *GNSSReferenceTime              `per:",optional"`
	GNSSReferenceLocation          *GNSSReferenceLocation          `per:",optional"`
	GNSSIonosphericModel           *GNSSIonosphericModel           `per:",optional"`
	GNSSEarthOrientationParameters *GNSSEarthOrientationParameters `per:",optional"`
}

type GNSSReferenceTime struct {
	_                         [0]struct{} `per:"extseq"`
	GNSSSystemTime            GNSSSystemTime
	ReferenceTimeUnc          *int64                        `per:",optional,range:0..127"`
	GNSSReferenceTimeForCells []GNSSReferenceTimeForOneCell `per:"SEQUENCE-OF,optional,size:1..16"`
}

type GNSSSystemTime struct {
	_                        [0]struct{} `per:"extseq"`
	GNSSTimeID               GNSSID
	GNSSDayNumber            int64         `per:",range:0..32767"`
	GNSSTimeOfDay            int64         `per:",range:0..86399"`
	GNSSTimeOfDayFracMsec    *int64        `per:",optional,range:0..999"`
	NotificationOfLeapSecond []bool        `per:",optional,size:2"`
	GPSTOWAssist             *GPSTOWAssist `per:",optional"`
}

type GPSTOWAssist struct {
	List []GPSTOWAssistElement `per:"SEQUENCE-OF,size:1..64"`
}

type GPSTOWAssistElement struct {
	_           [0]struct{} `per:"extseq"`
	SatelliteID int64       `per:",range:1..64"`
	TLMWord     int64       `per:",range:0..16383"`
	AntiSpoof   int64       `per:",range:0..1"`
	Alert       int64       `per:",range:0..1"`
	TLMRsvdBits int64       `per:",range:0..3"`
}

const (
	BSAlignTrue int64 = 0
)

type GNSSReferenceTimeForOneCell struct {
	_                [0]struct{} `per:"extseq"`
	NetworkTime      NetworkTime
	ReferenceTimeUnc int64  `per:",range:0..127"`
	BSAlign          *int64 `per:"ENUMERATED,optional,range:0..0"`
}

type NetworkTime struct {
	_                                        [0]struct{} `per:"extseq"`
	SecondsFromFrameStructureStart           int64       `per:",range:0..12533"`
	FractionalSecondsFromFrameStructureStart int64       `per:",range:0..3999999"`
	FrameDrift                               *int64      `per:",optional,range:-64..63"`
	CellID                                   NetworkTimeCellID
}

type NetworkTimeCellID struct {
	_     [0]struct{}             `per:"extseq"`
	EUTRA *NetworkTimeCellIDEUTRA `per:",choice:0,optional"`
	UTRA  *NetworkTimeCellIDUTRA  `per:",choice:1,optional"`
	GSM   *NetworkTimeCellIDGSM   `per:",choice:2,optional"`
}

type NetworkTimeCellIDEUTRA struct {
	_                 [0]struct{}               `per:"extseq"`
	PhysCellID        int64                     `per:",range:0..503"`
	CellGlobalIDEUTRA *CellGlobalIdEUTRAAndUTRA `per:",optional"`
	EARFCN            int64                     `per:",range:0..65535"`
}

type NetworkTimeCellIDUTRA struct {
	_                [0]struct{} `per:"extseq"`
	Mode             UTRAMode
	CellGlobalIDUTRA *CellGlobalIdEUTRAAndUTRA `per:",optional"`
	UARFCN           int64                     `per:",range:0..16383"`
}

type NetworkTimeCellIDGSM struct {
	_                 [0]struct{}        `per:"extseq"`
	BCCHCarrier       int64              `per:",range:0..1023"`
	BSIC              int64              `per:",range:0..63"`
	CellGlobalIDGERAN *CellGlobalIdGERAN `per:",optional"`
}

type GNSSReferenceLocation struct {
	_              [0]struct{} `per:"extseq"`
	ThreeDLocation EllipsoidPointWithAltitudeAndUncertaintyEllipsoid
}

type GNSSIonosphericModel struct {
	_              [0]struct{}              `per:"extseq"`
	KlobucharModel *KlobucharModelParameter `per:",optional"`
	NeQuickModel   *NeQuickModelParameter   `per:",optional"`
}

type KlobucharModelParameter struct {
	_      [0]struct{} `per:"extseq"`
	DataID []bool      `per:",size:2"`
	Alfa0  int64       `per:",range:-128..127"`
	Alfa1  int64       `per:",range:-128..127"`
	Alfa2  int64       `per:",range:-128..127"`
	Alfa3  int64       `per:",range:-128..127"`
	Beta0  int64       `per:",range:-128..127"`
	Beta1  int64       `per:",range:-128..127"`
	Beta2  int64       `per:",range:-128..127"`
	Beta3  int64       `per:",range:-128..127"`
}

type NeQuickModelParameter struct {
	_              [0]struct{} `per:"extseq"`
	AI0            int64       `per:",range:0..2047"`
	AI1            int64       `per:",range:-1024..1023"`
	AI2            int64       `per:",range:-8192..8191"`
	IonoStormFlag1 *int64      `per:",optional,range:0..1"`
	IonoStormFlag2 *int64      `per:",optional,range:0..1"`
	IonoStormFlag3 *int64      `per:",optional,range:0..1"`
	IonoStormFlag4 *int64      `per:",optional,range:0..1"`
	IonoStormFlag5 *int64      `per:",optional,range:0..1"`
}

type GNSSEarthOrientationParameters struct {
	_           [0]struct{} `per:"extseq"`
	TEOP        int64       `per:",range:0..65535"`
	PMX         int64       `per:",range:-1048576..1048575"`
	PMXDot      int64       `per:",range:-16384..16383"`
	PMY         int64       `per:",range:-1048576..1048575"`
	PMYDot      int64       `per:",range:-16384..16383"`
	DeltaUT1    int64       `per:",range:-1073741824..1073741823"`
	DeltaUT1Dot int64       `per:",range:-262144..262143"`
}

type GNSSGenericAssistData struct {
	List []GNSSGenericAssistDataElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSGenericAssistDataElement struct {
	_                           [0]struct{} `per:"extseq"`
	GNSSID                      GNSSID
	SBASID                      *SBASID                      `per:",optional"`
	GNSSTimeModels              *GNSSTimeModelList           `per:",optional"`
	GNSSDifferentialCorrections *GNSSDifferentialCorrections `per:",optional"`
	GNSSNavigationModel         *GNSSNavigationModel         `per:",optional"`
	GNSSRealTimeIntegrity       *GNSSRealTimeIntegrity       `per:",optional"`
	GNSSDataBitAssistance       *GNSSDataBitAssistance       `per:",optional"`
	GNSSAcquisitionAssistance   *GNSSAcquisitionAssistance   `per:",optional"`
	GNSSAlmanac                 *GNSSAlmanac                 `per:",optional"`
	GNSSUTCModel                *GNSSUTCModel                `per:",optional"`
	GNSSAuxiliaryInformation    *GNSSAuxiliaryInformation    `per:",optional"`
}

type GNSSTimeModelList struct {
	List []GNSSTimeModelElement `per:"SEQUENCE-OF,size:1..15"`
}

type GNSSTimeModelElement struct {
	_                    [0]struct{} `per:"extseq"`
	GNSSTimeModelRefTime int64       `per:",range:0..65535"`
	TA0                  int64       `per:",range:-67108864..67108863"`
	TA1                  *int64      `per:",optional,range:-4096..4095"`
	TA2                  *int64      `per:",optional,range:-64..63"`
	GNSSTOID             int64       `per:",range:1..15"`
	WeekNumber           *int64      `per:",optional,range:0..8191"`
	DeltaT               *int64      `per:",optional,range:-128..127"`
}

type GNSSDifferentialCorrections struct {
	_                [0]struct{} `per:"extseq"`
	DGNSSRefTime     int64       `per:",range:0..3599"`
	DGNSSSgnTypeList DGNSSSgnTypeList
}

type DGNSSSgnTypeList struct {
	List []DGNSSSgnTypeElement `per:"SEQUENCE-OF,size:1..3"`
}

type DGNSSSgnTypeElement struct {
	_                [0]struct{} `per:"extseq"`
	GNSSSignalID     GNSSSignalID
	GNSSStatusHealth int64 `per:",range:0..7"`
	DGNSSSatList     DGNSSSatList
}

type DGNSSSatList struct {
	List []DGNSSCorrectionsElement `per:"SEQUENCE-OF,size:1..64"`
}

type DGNSSCorrectionsElement struct {
	_                [0]struct{} `per:"extseq"`
	SVID             SVID
	IOD              []bool `per:",size:11"`
	UDRE             int64  `per:",range:0..3"`
	PseudoRangeCor   int64  `per:",range:-2047..2047"`
	RangeRateCor     int64  `per:",range:-127..127"`
	UDREGrowthRate   *int64 `per:",optional,range:0..7"`
	UDREValidityTime *int64 `per:",optional,range:0..7"`
}

type GNSSNavigationModel struct {
	_                   [0]struct{} `per:"extseq"`
	NonBroadcastIndFlag int64       `per:",range:0..1"`
	GNSSSatelliteList   GNSSNavModelSatelliteList
}

type GNSSNavModelSatelliteList struct {
	List []GNSSNavModelSatelliteElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSNavModelSatelliteElement struct {
	_              [0]struct{} `per:"extseq"`
	SVID           SVID
	SVHealth       []bool `per:",size:8"`
	IOD            []bool `per:",size:11"`
	GNSSClockModel GNSSClockModel
	GNSSOrbitModel GNSSOrbitModel
}

type GNSSClockModel struct {
	_                      [0]struct{}             `per:"extseq"`
	StandardClockModelList *StandardClockModelList `per:",choice:0,optional"`
	NAVClockModel          *NAVClockModel          `per:",choice:1,optional"`
	CNAVClockModel         *CNAVClockModel         `per:",choice:2,optional"`
	GLONASSClockModel      *GLONASSClockModel      `per:",choice:3,optional"`
	SBASClockModel         *SBASClockModel         `per:",choice:4,optional"`
}

type StandardClockModelList struct {
	List []StandardClockModelElement `per:"SEQUENCE-OF,size:1..2"`
}

type StandardClockModelElement struct {
	_            [0]struct{} `per:"extseq"`
	StanClockToc int64       `per:",range:0..16383"`
	StanClockAF2 int64       `per:",range:-32..31"`
	StanClockAF1 int64       `per:",range:-1048576..1048575"`
	StanClockAF0 int64       `per:",range:-1073741824..1073741823"`
	StanClockTgd *int64      `per:",optional,range:-512..511"`
	SISA         int64       `per:",range:0..255"`
	StanModelID  *int64      `per:",optional,range:0..1"`
}

type NAVClockModel struct {
	_      [0]struct{} `per:"extseq"`
	NavToc int64       `per:",range:0..37799"`
	NavAF2 int64       `per:",range:-128..127"`
	NavAF1 int64       `per:",range:-32768..32767"`
	NavAF0 int64       `per:",range:-2097152..2097151"`
	NavTgd int64       `per:",range:-128..127"`
}

type CNAVClockModel struct {
	_           [0]struct{} `per:"extseq"`
	CNAVToc     int64       `per:",range:0..2015"`
	CNAVTop     int64       `per:",range:0..2015"`
	CNAVURA0    int64       `per:",range:-16..15"`
	CNAVURA1    int64       `per:",range:0..7"`
	CNAVURA2    int64       `per:",range:0..7"`
	CNAVAf2     int64       `per:",range:-512..511"`
	CNAVAf1     int64       `per:",range:-524288..524287"`
	CNAVAf0     int64       `per:",range:-33554432..33554431"`
	CNAVTgd     int64       `per:",range:-4096..4095"`
	CNAVISCl1cp *int64      `per:",optional,range:-4096..4095"`
	CNAVISCl1cd *int64      `per:",optional,range:-4096..4095"`
	CNAVISCl1ca *int64      `per:",optional,range:-4096..4095"`
	CNAVISCl2c  *int64      `per:",optional,range:-4096..4095"`
	CNAVISCl5i5 *int64      `per:",optional,range:-4096..4095"`
	CNAVISCl5q5 *int64      `per:",optional,range:-4096..4095"`
}

type GLONASSClockModel struct {
	_           [0]struct{} `per:"extseq"`
	GloTau      int64       `per:",range:-2097152..2097151"`
	GloGamma    int64       `per:",range:-1024..1023"`
	GloDeltaTau *int64      `per:",optional,range:-16..15"`
}

type SBASClockModel struct {
	_        [0]struct{} `per:"extseq"`
	SBASTo   int64       `per:",range:0..5399"`
	SBASAgfo int64       `per:",range:-2048..2047"`
	SBASAgf1 int64       `per:",range:-128..127"`
}

type GNSSOrbitModel struct {
	_                [0]struct{}               `per:"extseq"`
	KeplerianSet     *NavModelKeplerianSet     `per:",choice:0,optional"`
	NAVKeplerianSet  *NavModelNAVKeplerianSet  `per:",choice:1,optional"`
	CNAVKeplerianSet *NavModelCNAVKeplerianSet `per:",choice:2,optional"`
	GLONASSECEF      *NavModelGLONASSECEF      `per:",choice:3,optional"`
	SBASECEF         *NavModelSBASECEF         `per:",choice:4,optional"`
}

type NavModelKeplerianSet struct {
	_                [0]struct{} `per:"extseq"`
	KeplerToe        int64       `per:",range:0..16383"`
	KeplerW          int64       `per:",range:-2147483648..2147483647"`
	KeplerDeltaN     int64       `per:",range:-32768..32767"`
	KeplerM0         int64       `per:",range:-2147483648..2147483647"`
	KeplerOmegaDot   int64       `per:",range:-8388608..8388607"`
	KeplerE          int64       `per:",range:0..4294967295"`
	KeplerIDot       int64       `per:",range:-8192..8191"`
	KeplerAPowerHalf int64       `per:",range:0..4294967295"`
	KeplerI0         int64       `per:",range:-2147483648..2147483647"`
	KeplerOmega0     int64       `per:",range:-2147483648..2147483647"`
	KeplerCrs        int64       `per:",range:-32768..32767"`
	KeplerCis        int64       `per:",range:-32768..32767"`
	KeplerCus        int64       `per:",range:-32768..32767"`
	KeplerCrc        int64       `per:",range:-32768..32767"`
	KeplerCic        int64       `per:",range:-32768..32767"`
	KeplerCuc        int64       `per:",range:-32768..32767"`
}

type NavModelNAVKeplerianSet struct {
	_             [0]struct{}  `per:"extseq"`
	NavURA        int64        `per:",range:0..15"`
	NavFitFlag    int64        `per:",range:0..1"`
	NavToe        int64        `per:",range:0..37799"`
	NavOmega      int64        `per:",range:-2147483648..2147483647"`
	NavDeltaN     int64        `per:",range:-32768..32767"`
	NavM0         int64        `per:",range:-2147483648..2147483647"`
	NavOmegaADot  int64        `per:",range:-8388608..8388607"`
	NavE          int64        `per:",range:0..4294967295"`
	NavIDot       int64        `per:",range:-8192..8191"`
	NavAPowerHalf int64        `per:",range:0..4294967295"`
	NavI0         int64        `per:",range:-2147483648..2147483647"`
	NavOmegaA0    int64        `per:",range:-2147483648..2147483647"`
	NavCrs        int64        `per:",range:-32768..32767"`
	NavCis        int64        `per:",range:-32768..32767"`
	NavCus        int64        `per:",range:-32768..32767"`
	NavCrc        int64        `per:",range:-32768..32767"`
	NavCic        int64        `per:",range:-32768..32767"`
	NavCuc        int64        `per:",range:-32768..32767"`
	AddNAVParam   *AddNAVParam `per:",optional"`
}

type AddNAVParam struct {
	EphemCodeOnL2 int64 `per:",range:0..3"`
	EphemL2Pflag  int64 `per:",range:0..1"`
	EphemSF1Rsvd  EphemSF1Rsvd
	EphemAODA     int64 `per:",range:0..31"`
}

type EphemSF1Rsvd struct {
	Reserved1 int64 `per:",range:0..8388607"`
	Reserved2 int64 `per:",range:0..16777215"`
	Reserved3 int64 `per:",range:0..16777215"`
	Reserved4 int64 `per:",range:0..65535"`
}

type NavModelCNAVKeplerianSet struct {
	_                 [0]struct{} `per:"extseq"`
	CNAVTop           int64       `per:",range:0..2015"`
	CNAVURAIndex      int64       `per:",range:-16..15"`
	CNAVDeltaA        int64       `per:",range:-33554432..33554431"`
	CNAVAdot          int64       `per:",range:-16777216..16777215"`
	CNAVDeltaNo       int64       `per:",range:-65536..65535"`
	CNAVDeltaNoDot    int64       `per:",range:-4194304..4194303"`
	CNAVMo            int64       `per:",range:-4294967296..4294967295"`
	CNAVE             int64       `per:",range:0..8589934591"`
	CNAVOmega         int64       `per:",range:-4294967296..4294967295"`
	CNAVOMEGA0        int64       `per:",range:-4294967296..4294967295"`
	CNAVDeltaOmegaDot int64       `per:",range:-65536..65535"`
	CNAVIo            int64       `per:",range:-4294967296..4294967295"`
	CNAVIoDot         int64       `per:",range:-16384..16383"`
	CNAVCis           int64       `per:",range:-32768..32767"`
	CNAVCic           int64       `per:",range:-32768..32767"`
	CNAVCrs           int64       `per:",range:-8388608..8388607"`
	CNAVCrc           int64       `per:",range:-8388608..8388607"`
	CNAVCus           int64       `per:",range:-1048576..1048575"`
	CNAVCuc           int64       `per:",range:-1048576..1048575"`
}

type NavModelGLONASSECEF struct {
	_          [0]struct{} `per:"extseq"`
	GloEn      int64       `per:",range:0..31"`
	GloP1      []bool      `per:",size:2"`
	GloP2      bool
	GloM       int64 `per:",range:0..3"`
	GloX       int64 `per:",range:-67108864..67108863"`
	GloXdot    int64 `per:",range:-8388608..8388607"`
	GloXdotdot int64 `per:",range:-16..15"`
	GloY       int64 `per:",range:-67108864..67108863"`
	GloYdot    int64 `per:",range:-8388608..8388607"`
	GloYdotdot int64 `per:",range:-16..15"`
	GloZ       int64 `per:",range:-67108864..67108863"`
	GloZdot    int64 `per:",range:-8388608..8388607"`
	GloZdotdot int64 `per:",range:-16..15"`
}

type NavModelSBASECEF struct {
	_            [0]struct{} `per:"extseq"`
	SBASTo       *int64      `per:",optional,range:0..5399"`
	SBASAccuracy []bool      `per:",size:4"`
	SBASXg       int64       `per:",range:-536870912..536870911"`
	SBASYg       int64       `per:",range:-536870912..536870911"`
	SBASZg       int64       `per:",range:-16777216..16777215"`
	SBASXgDot    int64       `per:",range:-65536..65535"`
	SBASYgDot    int64       `per:",range:-65536..65535"`
	SBASZgDot    int64       `per:",range:-131072..131071"`
	SBASXgDotDot int64       `per:",range:-512..511"`
	SBAGYgDotDot int64       `per:",range:-512..511"`
	SBASZgDotDot int64       `per:",range:-512..511"`
}

type GNSSRealTimeIntegrity struct {
	_                 [0]struct{} `per:"extseq"`
	GNSSBadSignalList GNSSBadSignalList
}

type GNSSBadSignalList struct {
	List []BadSignalElement `per:"SEQUENCE-OF,size:1..64"`
}

type BadSignalElement struct {
	_           [0]struct{} `per:"extseq"`
	BadSVID     SVID
	BadSignalID *GNSSSignalIDs `per:",optional"`
}

type GNSSDataBitAssistance struct {
	_                   [0]struct{} `per:"extseq"`
	GNSSTOD             int64       `per:",range:0..3599"`
	GNSSTODFrac         *int64      `per:",optional,range:0..999"`
	GNSSDataBitsSatList GNSSDataBitsSatList
}

type GNSSDataBitsSatList struct {
	List []GNSSDataBitsSatElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSDataBitsSatElement struct {
	_                   [0]struct{} `per:"extseq"`
	SVID                SVID
	GNSSDataBitsSgnList GNSSDataBitsSgnList
}

type GNSSDataBitsSgnList struct {
	List []GNSSDataBitsSgnElement `per:"SEQUENCE-OF,size:1..8"`
}

type GNSSDataBitsSgnElement struct {
	_              [0]struct{} `per:"extseq"`
	GNSSSignalType GNSSSignalID
	GNSSDataBits   []bool `per:",size:1..1024"`
}

type GNSSAcquisitionAssistance struct {
	_                         [0]struct{} `per:"extseq"`
	GNSSSignalID              GNSSSignalID
	GNSSAcquisitionAssistList GNSSAcquisitionAssistList
}

type GNSSAcquisitionAssistList struct {
	List []GNSSAcquisitionAssistElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSAcquisitionAssistElement struct {
	_                     [0]struct{} `per:"extseq"`
	SVID                  SVID
	Doppler0              int64 `per:",range:-2048..2047"`
	Doppler1              int64 `per:",range:0..63"`
	DopplerUncertainty    int64 `per:",range:0..4"`
	CodePhase             int64 `per:",range:0..1022"`
	IntCodePhase          int64 `per:",range:0..127"`
	CodePhaseSearchWindow int64 `per:",range:0..31"`
	Azimuth               int64 `per:",range:0..511"`
	Elevation             int64 `per:",range:0..127"`
}

type GNSSAlmanac struct {
	_                       [0]struct{} `per:"extseq"`
	WeekNumber              *int64      `per:",optional,range:0..255"`
	TOA                     *int64      `per:",optional,range:0..255"`
	IODA                    *int64      `per:",optional,range:0..3"`
	CompleteAlmanacProvided bool
	GNSSAlmanacList         GNSSAlmanacList
}

type GNSSAlmanacList struct {
	List []GNSSAlmanacElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSAlmanacElement struct {
	_                       [0]struct{}                 `per:"extseq"`
	KeplerianAlmanacSet     *AlmanacKeplerianSet        `per:",choice:0,optional"`
	KeplerianNAVAlmanac     *AlmanacNAVKeplerianSet     `per:",choice:1,optional"`
	KeplerianReducedAlmanac *AlmanacReducedKeplerianSet `per:",choice:2,optional"`
	KeplerianMidiAlmanac    *AlmanacMidiAlmanacSet      `per:",choice:3,optional"`
	KeplerianGLONASS        *AlmanacGLONASSAlmanacSet   `per:",choice:4,optional"`
	ECEFSBASAlmanac         *AlmanacECEFSBASAlmanacSet  `per:",choice:5,optional"`
}

type AlmanacKeplerianSet struct {
	_                    [0]struct{} `per:"extseq"`
	SVID                 SVID
	KepAlmanacE          int64  `per:",range:0..2047"`
	KepAlmanacDeltaI     int64  `per:",range:-1024..1023"`
	KepAlmanacOmegaDot   int64  `per:",range:-1024..1023"`
	KepSVStatusINAV      []bool `per:",size:4"`
	KepSVStatusFNAV      []bool `per:",optional,size:2"`
	KepAlmanacAPowerHalf int64  `per:",range:-4096..4095"`
	KepAlmanacOmega0     int64  `per:",range:-32768..32767"`
	KepAlmanacW          int64  `per:",range:-32768..32767"`
	KepAlmanacM0         int64  `per:",range:-32768..32767"`
	KepAlmanacAF0        int64  `per:",range:-32768..32767"`
	KepAlmanacAF1        int64  `per:",range:-4096..4095"`
}

type AlmanacNAVKeplerianSet struct {
	_              [0]struct{} `per:"extseq"`
	SVID           SVID
	NavAlmE        int64 `per:",range:0..65535"`
	NavAlmDeltaI   int64 `per:",range:-32768..32767"`
	NavAlmOMEGADOT int64 `per:",range:-32768..32767"`
	NavAlmSVHealth int64 `per:",range:0..255"`
	NavAlmSqrtA    int64 `per:",range:0..16777215"`
	NavAlmOMEGAo   int64 `per:",range:-8388608..8388607"`
	NavAlmOmega    int64 `per:",range:-8388608..8388607"`
	NavAlmMo       int64 `per:",range:-8388608..8388607"`
	NavAlmaf0      int64 `per:",range:-1024..1023"`
	NavAlmaf1      int64 `per:",range:-1024..1023"`
}

type AlmanacReducedKeplerianSet struct {
	_              [0]struct{} `per:"extseq"`
	SVID           SVID
	RedAlmDeltaA   int64 `per:",range:-128..127"`
	RedAlmOmega0   int64 `per:",range:-64..63"`
	RedAlmPhi0     int64 `per:",range:-64..63"`
	RedAlmL1Health bool
	RedAlmL2Health bool
	RedAlmL5Health bool
}

type AlmanacMidiAlmanacSet struct {
	_               [0]struct{} `per:"extseq"`
	SVID            SVID
	MidiAlmE        int64 `per:",range:0..2047"`
	MidiAlmDeltaI   int64 `per:",range:-1024..1023"`
	MidiAlmOmegaDot int64 `per:",range:-1024..1023"`
	MidiAlmSqrtA    int64 `per:",range:0..131071"`
	MidiAlmOmega0   int64 `per:",range:-32768..32767"`
	MidiAlmOmega    int64 `per:",range:-32768..32767"`
	MidiAlmMo       int64 `per:",range:-32768..32767"`
	MidiAlmaf0      int64 `per:",range:-1024..1023"`
	MidiAlmaf1      int64 `per:",range:-512..511"`
	MidiAlmL1Health bool
	MidiAlmL2Health bool
	MidiAlmL5Health bool
}

type AlmanacGLONASSAlmanacSet struct {
	_                [0]struct{} `per:"extseq"`
	GloAlmNA         int64       `per:",range:1..1461"`
	GloAlmnA         int64       `per:",range:1..24"`
	GloAlmHA         int64       `per:",range:0..31"`
	GloAlmLambdaA    int64       `per:",range:-1048576..1048575"`
	GloAlmtlambdaA   int64       `per:",range:0..2097151"`
	GloAlmDeltaIa    int64       `per:",range:-131072..131071"`
	GloAlmDeltaTA    int64       `per:",range:-2097152..2097151"`
	GloAlmDeltaTdotA int64       `per:",range:-64..63"`
	GloAlmEpsilonA   int64       `per:",range:0..32767"`
	GloAlmOmegaA     int64       `per:",range:-32768..32767"`
	GloAlmTauA       int64       `per:",range:-512..511"`
	GloAlmCA         int64       `per:",range:0..1"`
	GloAlmMA         []bool      `per:",optional,size:2"`
}

type AlmanacECEFSBASAlmanacSet struct {
	_             [0]struct{} `per:"extseq"`
	SBASAlmDataID int64       `per:",range:0..3"`
	SVID          SVID
	SBASAlmHealth []bool `per:",size:8"`
	SBASAlmXg     int64  `per:",range:-16384..16383"`
	SBASAlmYg     int64  `per:",range:-16384..16383"`
	SBASAlmZg     int64  `per:",range:-256..255"`
	SBASAlmXgdot  int64  `per:",range:-4..3"`
	SBASAlmYgDot  int64  `per:",range:-4..3"`
	SBASAlmZgDot  int64  `per:",range:-8..7"`
	SBASAlmTo     int64  `per:",range:0..2047"`
}

type GNSSUTCModel struct {
	_         [0]struct{}   `per:"extseq"`
	UTCModel1 *UTCModelSet1 `per:",choice:0,optional"`
	UTCModel2 *UTCModelSet2 `per:",choice:1,optional"`
	UTCModel3 *UTCModelSet3 `per:",choice:2,optional"`
	UTCModel4 *UTCModelSet4 `per:",choice:3,optional"`
}

type UTCModelSet1 struct {
	_                [0]struct{} `per:"extseq"`
	GNSSUtcA1        int64       `per:",range:-8388608..8388607"`
	GNSSUtcA0        int64       `per:",range:-2147483648..2147483647"`
	GNSSUtcTot       int64       `per:",range:0..255"`
	GNSSUtcWNt       int64       `per:",range:0..255"`
	GNSSUtcDeltaTls  int64       `per:",range:-128..127"`
	GNSSUtcWNlsf     int64       `per:",range:0..255"`
	GNSSUtcDN        int64       `per:",range:-128..127"`
	GNSSUtcDeltaTlsf int64       `per:",range:-128..127"`
}

type UTCModelSet2 struct {
	_            [0]struct{} `per:"extseq"`
	UTCA0        int64       `per:",range:-32768..32767"`
	UTCA1        int64       `per:",range:-4096..4095"`
	UTCA2        int64       `per:",range:-64..63"`
	UTCDeltaTls  int64       `per:",range:-128..127"`
	UTCTot       int64       `per:",range:0..65535"`
	UTCWNot      int64       `per:",range:0..8191"`
	UTCWNlsf     int64       `per:",range:0..255"`
	UTCDN        []bool      `per:",size:4"`
	UTCDeltaTlsf int64       `per:",range:-128..127"`
}

type UTCModelSet3 struct {
	_    [0]struct{} `per:"extseq"`
	NA   int64       `per:",range:1..1461"`
	TauC int64       `per:",range:-2147483648..2147483647"`
	B1   *int64      `per:",optional,range:-1024..1023"`
	B2   *int64      `per:",optional,range:-512..511"`
	KP   []bool      `per:",optional,size:2"`
}

type UTCModelSet4 struct {
	_             [0]struct{} `per:"extseq"`
	UTCA1wnt      int64       `per:",range:-8388608..8388607"`
	UTCA0wnt      int64       `per:",range:-2147483648..2147483647"`
	UTCTot        int64       `per:",range:0..255"`
	UTCWNt        int64       `per:",range:0..255"`
	UTCDeltaTls   int64       `per:",range:-128..127"`
	UTCWNlsf      int64       `per:",range:0..255"`
	UTCDN         int64       `per:",range:-128..127"`
	UTCDeltaTlsf  int64       `per:",range:-128..127"`
	UTCStandardID int64       `per:",range:0..7"`
}

type GNSSAuxiliaryInformation struct {
	_             [0]struct{}        `per:"extseq"`
	GNSSIDGPS     *GNSSIDGPSList     `per:",choice:0,optional"`
	GNSSIDGLONASS *GNSSIDGLONASSList `per:",choice:1,optional"`
}

type GNSSIDGPSList struct {
	List []GNSSIDGPSSatElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSIDGPSSatElement struct {
	_                [0]struct{} `per:"extseq"`
	SVID             SVID
	SignalsAvailable GNSSSignalIDs
}

type GNSSIDGLONASSList struct {
	List []GNSSIDGLONASSSatElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSIDGLONASSSatElement struct {
	_                [0]struct{} `per:"extseq"`
	SVID             SVID
	SignalsAvailable GNSSSignalIDs
	ChannelNumber    *int64 `per:",optional,range:-7..13"`
}
