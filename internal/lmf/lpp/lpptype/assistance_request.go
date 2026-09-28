// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/per"

type CommonIEsRequestAssistanceData struct {
	_             [0]struct{} `per:"extseq"`
	PrimaryCellID *ECGI       `per:",optional"`
}

type AGNSSRequestAssistanceData struct {
	_                        [0]struct{}               `per:"extseq"`
	GNSSCommonAssistDataReq  *GNSSCommonAssistDataReq  `per:",optional"`
	GNSSGenericAssistDataReq *GNSSGenericAssistDataReq `per:",optional"`
}

type GNSSCommonAssistDataReq struct {
	_                                 [0]struct{}                        `per:"extseq"`
	GNSSReferenceTimeReq              *GNSSReferenceTimeReq              `per:",optional"`
	GNSSReferenceLocationReq          *GNSSReferenceLocationReq          `per:",optional"`
	GNSSIonosphericModelReq           *GNSSIonosphericModelReq           `per:",optional"`
	GNSSEarthOrientationParametersReq *GNSSEarthOrientationParametersReq `per:",optional"`
}

type GNSSReferenceTimeReq struct {
	_                   [0]struct{} `per:"extseq"`
	GNSSTimeReqPrefList []GNSSID    `per:"SEQUENCE-OF,size:1..8"`
	GPSTOWAssistReq     *bool       `per:",optional"`
	NotOfLeapSecReq     *bool       `per:",optional"`
}

type GNSSReferenceLocationReq struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSIonosphericModelReq struct {
	_                 [0]struct{} `per:"extseq"`
	KlobucharModelReq []bool      `per:",optional,size:2"`
	NeQuickModelReq   *per.Null   `per:",optional"`
}

type GNSSEarthOrientationParametersReq struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSGenericAssistDataReq struct {
	List []GNSSGenericAssistDataReqElement `per:"SEQUENCE-OF,size:1..16"`
}

type GNSSGenericAssistDataReqElement struct {
	_                              [0]struct{} `per:"extseq"`
	GNSSID                         GNSSID
	SBASID                         *SBASID                         `per:",optional"`
	GNSSTimeModelsReq              *GNSSTimeModelListReq           `per:",optional"`
	GNSSDifferentialCorrectionsReq *GNSSDifferentialCorrectionsReq `per:",optional"`
	GNSSNavigationModelReq         *GNSSNavigationModelReq         `per:",optional"`
	GNSSRealTimeIntegrityReq       *GNSSRealTimeIntegrityReq       `per:",optional"`
	GNSSDataBitAssistanceReq       *GNSSDataBitAssistanceReq       `per:",optional"`
	GNSSAcquisitionAssistanceReq   *GNSSAcquisitionAssistanceReq   `per:",optional"`
	GNSSAlmanacReq                 *GNSSAlmanacReq                 `per:",optional"`
	GNSSUTCModelReq                *GNSSUTCModelReq                `per:",optional"`
	GNSSAuxiliaryInformationReq    *GNSSAuxiliaryInformationReq    `per:",optional"`
}

type GNSSTimeModelListReq struct {
	List []GNSSTimeModelElementReq `per:"SEQUENCE-OF,size:1..15"`
}

type GNSSTimeModelElementReq struct {
	_            [0]struct{} `per:"extseq"`
	GNSSTOIDsReq int64       `per:",range:1..15"`
	DeltaTReq    bool
}

type GNSSDifferentialCorrectionsReq struct {
	_                    [0]struct{} `per:"extseq"`
	DGNSSSignalsReq      GNSSSignalIDs
	DGNSSValidityTimeReq bool
}

type GNSSNavigationModelReq struct {
	_             [0]struct{}        `per:"extseq"`
	StoredNavList *StoredNavListInfo `per:",choice:0,optional"`
	ReqNavList    *ReqNavListInfo    `per:",choice:1,optional"`
}

type StoredNavListInfo struct {
	_                      [0]struct{}             `per:"extseq"`
	GNSSWeekOrDay          int64                   `per:",range:0..4095"`
	GNSSToe                int64                   `per:",range:0..255"`
	TToeLimit              int64                   `per:",range:0..15"`
	SatListRelatedDataList *SatListRelatedDataList `per:",optional"`
}

type SatListRelatedDataList struct {
	List []SatListRelatedDataElement `per:"SEQUENCE-OF,size:1..64"`
}

type SatListRelatedDataElement struct {
	_            [0]struct{} `per:"extseq"`
	SVID         SVID
	IOD          []bool `per:",size:11"`
	ClockModelID *int64 `per:",optional,range:1..8"`
	OrbitModelID *int64 `per:",optional,range:1..8"`
}

type ModelID struct {
	Value int64 `per:",range:1..8"`
}

type ReqNavListInfo struct {
	_                    [0]struct{} `per:"extseq"`
	SVReqList            []bool      `per:",size:64"`
	ClockModelIDPrefList []ModelID   `per:"SEQUENCE-OF,optional,size:1..8"`
	OrbitModelIDPrefList []ModelID   `per:"SEQUENCE-OF,optional,size:1..8"`
	AddNavparamReq       *bool       `per:",optional"`
}

type GNSSRealTimeIntegrityReq struct {
	_ [0]struct{} `per:"extseq"`
}

type GNSSDataBitAssistanceReq struct {
	_               [0]struct{} `per:"extseq"`
	GNSSTODReq      int64       `per:",range:0..3599"`
	GNSSTODFracReq  *int64      `per:",optional,range:0..999"`
	DataBitInterval int64       `per:",range:0..15"`
	GNSSSignalType  GNSSSignalIDs
	GNSSDataBitsReq *GNSSDataBitsReqSatList `per:",optional"`
}

type GNSSDataBitsReqSatList struct {
	List []GNSSDataBitsReqSatElement `per:"SEQUENCE-OF,size:1..64"`
}

type GNSSDataBitsReqSatElement struct {
	_    [0]struct{} `per:"extseq"`
	SVID SVID
}

type GNSSAcquisitionAssistanceReq struct {
	_               [0]struct{} `per:"extseq"`
	GNSSSignalIDReq GNSSSignalID
}

type GNSSAlmanacReq struct {
	_       [0]struct{} `per:"extseq"`
	ModelID *int64      `per:",optional,range:1..8"`
}

type GNSSUTCModelReq struct {
	_       [0]struct{} `per:"extseq"`
	ModelID *int64      `per:",optional,range:1..8"`
}

type GNSSAuxiliaryInformationReq struct {
	_ [0]struct{} `per:"extseq"`
}
