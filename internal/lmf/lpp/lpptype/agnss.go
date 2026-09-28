// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

// =====================================================================
// A-GNSS-RequestLocationInformation (TS 37.355 §6.5.2.7)
// =====================================================================

//	A-GNSS-RequestLocationInformation ::= SEQUENCE {
//	    gnss-PositioningInstructions  GNSS-PositioningInstructions,
//	    ...
//	}
type AGNSSRequestLocationInformation struct {
	_                           [0]struct{} `per:"extseq"`
	GNSSPositioningInstructions GNSSPositioningInstructions
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
	GNSSMethods               GNSSIDBitmap
	FineTimeAssistanceMeasReq bool
	ADRMeasReq                bool
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
	GNSSSignalMeasurementInformation *GNSSSignalMeasurementInformation `per:",optional"`
	GNSSLocationInformation          *GNSSLocationInformation          `per:",optional"`
	GNSSError                        *AGNSSError                       `per:",optional"`
}

type GNSSSignalMeasurementInformation struct {
	_                        [0]struct{} `per:"extseq"`
	MeasurementReferenceTime MeasurementReferenceTime
	GNSSMeasurementList      GNSSMeasurementList
}

// =====================================================================
// MeasurementReferenceTime (TS 37.355 §6.5.2.6)
// =====================================================================

//	MeasurementReferenceTime ::= SEQUENCE {
//	    gnss-TOD-msec   INTEGER (0..3599999),
//	    gnss-TOD-frac   INTEGER (0..3999) OPTIONAL,
//	    gnss-TOD-unc   INTEGER (0..127) OPTIONAL,
//	    gnss-TimeID    GNSS-ID,
//	    networkTime    CHOICE { eUTRA, uTRA, gSM, ..., nbIoT-r14, nr-r15 } OPTIONAL,
//	    ...
//	}
type MeasurementReferenceTime struct {
	_           [0]struct{} `per:"extseq"`
	GNSSTODMsec int64       `per:",range:0..3599999"`
	GNSSTODFrac *int64      `per:",optional,range:0..3999"`
	GNSSTODUnc  *int64      `per:",optional,range:0..127"`
	GNSSTimeID  GNSSID
	NetworkTime *MeasurementReferenceTimeNetworkTime `per:",optional"`
}

type MeasurementReferenceTimeNetworkTime struct {
	_     [0]struct{}                               `per:"extseq"`
	EUTRA *MeasurementReferenceTimeNetworkTimeEUTRA `per:",choice:0,optional"`
	UTRA  *MeasurementReferenceTimeNetworkTimeUTRA  `per:",choice:1,optional"`
	GSM   *MeasurementReferenceTimeNetworkTimeGSM   `per:",choice:2,optional"`
}

type MeasurementReferenceTimeNetworkTimeEUTRA struct {
	_                 [0]struct{}               `per:"extseq"`
	PhysCellID        int64                     `per:",range:0..503"`
	CellGlobalID      *CellGlobalIdEUTRAAndUTRA `per:",optional"`
	SystemFrameNumber []bool                    `per:",size:10"`
}

type MeasurementReferenceTimeNetworkTimeUTRA struct {
	_                          [0]struct{} `per:"extseq"`
	Mode                       UTRAMode
	CellGlobalID               *CellGlobalIdEUTRAAndUTRA `per:",optional"`
	ReferenceSystemFrameNumber int64                     `per:",range:0..4095"`
}

type UTRAMode struct {
	FDD *UTRAModeFDD `per:",choice:0,optional"`
	TDD *UTRAModeTDD `per:",choice:1,optional"`
}

type UTRAModeFDD struct {
	_                [0]struct{} `per:"extseq"`
	PrimaryCPICHInfo int64       `per:",range:0..511"`
}

type UTRAModeTDD struct {
	_              [0]struct{} `per:"extseq"`
	CellParameters int64       `per:",range:0..127"`
}

type MeasurementReferenceTimeNetworkTimeGSM struct {
	_              [0]struct{}        `per:"extseq"`
	BCCHCarrier    int64              `per:",range:0..1023"`
	BSIC           int64              `per:",range:0..63"`
	CellGlobalID   *CellGlobalIdGERAN `per:",optional"`
	ReferenceFrame GSMReferenceFrame
	DeltaGNSSTOD   *int64 `per:",optional,range:0..127"`
}

type GSMReferenceFrame struct {
	_              [0]struct{} `per:"extseq"`
	ReferenceFN    int64       `per:",range:0..65535"`
	ReferenceFNMSB *int64      `per:",optional,range:0..63"`
}

type GNSSMeasurementList struct {
	List []GNSSMeasurementForOneGNSS `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSMeasurementForOneGNSS struct {
	_               [0]struct{} `per:"extseq"`
	GNSSID          GNSSID
	GNSSSgnMeasList GNSSSgnMeasList
}

type GNSSSgnMeasList struct {
	List []GNSSSgnMeasElement `per:"SEQUENCE-OF,size:1..8"`
}

type GNSSSgnMeasElement struct {
	_                      [0]struct{} `per:"extseq"`
	GNSSSignalID           GNSSSignalID
	GNSSCodePhaseAmbiguity *int64 `per:",optional,range:0..127"`
	GNSSSatMeasList        GNSSSatMeasList
}

type GNSSSatMeasList struct {
	List []GNSSSatMeasElement `per:"SEQUENCE-OF,size:1..64"`
}

type MpathDet int64

const (
	MpathDetNotMeasured MpathDet = 0
	MpathDetLow         MpathDet = 1
	MpathDetMedium      MpathDet = 2
	MpathDetHigh        MpathDet = 3
)

type GNSSSatMeasElement struct {
	_                 [0]struct{} `per:"extseq"`
	SVID              SVID
	CNo               int64    `per:",range:0..63"`
	MpathDet          MpathDet `per:"ENUMERATED,range:0..3,...,extvalues:0"`
	CarrierQualityInd *int64   `per:",optional,range:0..3"`
	CodePhase         int64    `per:",range:0..2097151"`
	IntegerCodePhase  *int64   `per:",optional,range:0..127"`
	CodePhaseRMSError int64    `per:",range:0..63"`
	Doppler           *int64   `per:",optional,range:-32768..32767"`
	ADR               *int64   `per:",optional,range:0..33554431"`
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
	AGNSSList                GNSSIDBitmap
}
