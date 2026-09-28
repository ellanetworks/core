// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lpptype holds hand-written, per-tagged Go structs that mirror the
// 3GPP TS 37.355 (Rel-18) LPP ASN.1 definitions reachable from LPP-Message.
//
// The PER codec is github.com/ellanetworks/core/per using the Unaligned
// variant (BASIC-PER Unaligned, per TS 37.355 §7). Every type models the root
// components of its ASN.1 definition; extension additions are skipped on
// decode and never encoded. Tag conventions:
//   - SEQUENCE: Go struct, optional fields tagged `per:",optional"`
//   - CHOICE: Go struct with pointer fields tagged `per:",choice:N,optional"`
//   - Extensible types: placeholder field tagged `per:"extseq"`
//   - ENUMERATED: int64 with `per:"ENUMERATED,range:0..N"`, extension values
//     decode as N+1+k
//   - BIT STRING: []bool with `per:",size:lb..ub"`
//   - INTEGER: int64 with `per:",range:lb..ub"`
package lpptype

import "github.com/ellanetworks/core/per"

//go:generate sh -c "rm -f per_gen.go && go run github.com/ellanetworks/core/cmd/pergen"

type LPPMessage struct {
	TransactionID   *LPPTransactionID `per:",optional"`
	EndTransaction  bool
	SequenceNumber  *int64           `per:",optional,range:0..255"`
	Acknowledgement *Acknowledgement `per:",optional"`
	LPPMessageBody  *LPPMessageBody  `per:",optional"`
}

type LPPTransactionID struct {
	_                 [0]struct{} `per:"extseq"`
	Initiator         Initiator
	TransactionNumber int64 `per:",range:0..255"`
}

const (
	InitiatorLocationServer int64 = 0
	InitiatorTargetDevice   int64 = 1
)

type Initiator struct {
	Value int64 `per:"ENUMERATED,range:0..1,..."`
}

type Acknowledgement struct {
	AckRequested bool
	AckIndicator *int64 `per:",optional,range:0..255"`
}

type LPPMessageBody struct {
	C1                    *LPPMessageBodyC1 `per:",choice:0,optional"`
	MessageClassExtension *per.Null         `per:",choice:1,optional"`
}

const (
	LPPMessageBodyC1PresentNothing int = iota
	LPPMessageBodyC1PresentRequestCapabilities
	LPPMessageBodyC1PresentProvideCapabilities
	LPPMessageBodyC1PresentRequestAssistanceData
	LPPMessageBodyC1PresentProvideAssistanceData
	LPPMessageBodyC1PresentRequestLocationInformation
	LPPMessageBodyC1PresentProvideLocationInformation
	LPPMessageBodyC1PresentAbort
	LPPMessageBodyC1PresentError
)

type LPPMessageBodyC1 struct {
	RequestCapabilities        *RequestCapabilities        `per:",choice:0,optional"`
	ProvideCapabilities        *ProvideCapabilities        `per:",choice:1,optional"`
	RequestAssistanceData      *RequestAssistanceData      `per:",choice:2,optional"`
	ProvideAssistanceData      *ProvideAssistanceData      `per:",choice:3,optional"`
	RequestLocationInformation *RequestLocationInformation `per:",choice:4,optional"`
	ProvideLocationInformation *ProvideLocationInformation `per:",choice:5,optional"`
	Abort                      *Abort                      `per:",choice:6,optional"`
	Error                      *Error                      `per:",choice:7,optional"`
	Spare7                     *per.Null                   `per:",choice:8,optional"`
	Spare6                     *per.Null                   `per:",choice:9,optional"`
	Spare5                     *per.Null                   `per:",choice:10,optional"`
	Spare4                     *per.Null                   `per:",choice:11,optional"`
	Spare3                     *per.Null                   `per:",choice:12,optional"`
	Spare2                     *per.Null                   `per:",choice:13,optional"`
	Spare1                     *per.Null                   `per:",choice:14,optional"`
	Spare0                     *per.Null                   `per:",choice:15,optional"`
}

type RequestCapabilities struct {
	CriticalExtensions RequestCapabilitiesCriticalExtensions
}

type RequestCapabilitiesCriticalExtensions struct {
	C1                       *RequestCapabilitiesCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                `per:",choice:1,optional"`
}

type RequestCapabilitiesCriticalExtensionsC1 struct {
	RequestCapabilitiesR9 *RequestCapabilitiesR9IEs `per:",choice:0,optional"`
	Spare3                *per.Null                 `per:",choice:1,optional"`
	Spare2                *per.Null                 `per:",choice:2,optional"`
	Spare1                *per.Null                 `per:",choice:3,optional"`
}

type RequestCapabilitiesR9IEs struct {
	_                            [0]struct{}                   `per:"extseq"`
	CommonIEsRequestCapabilities *CommonIEsRequestCapabilities `per:",optional"`
	AGNSSRequestCapabilities     *AGNSSRequestCapabilities     `per:",optional"`
	OTDOARequestCapabilities     *OTDOARequestCapabilities     `per:",optional"`
	ECIDRequestCapabilities      *ECIDRequestCapabilities      `per:",optional"`
	EPDURequestCapabilities      *EPDUSequence                 `per:",optional"`
}

type ProvideCapabilities struct {
	CriticalExtensions ProvideCapabilitiesCriticalExtensions
}

type ProvideCapabilitiesCriticalExtensions struct {
	C1                       *ProvideCapabilitiesCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                `per:",choice:1,optional"`
}

type ProvideCapabilitiesCriticalExtensionsC1 struct {
	ProvideCapabilitiesR9 *ProvideCapabilitiesR9IEs `per:",choice:0,optional"`
	Spare3                *per.Null                 `per:",choice:1,optional"`
	Spare2                *per.Null                 `per:",choice:2,optional"`
	Spare1                *per.Null                 `per:",choice:3,optional"`
}

type ProvideCapabilitiesR9IEs struct {
	_                            [0]struct{}                   `per:"extseq"`
	CommonIEsProvideCapabilities *CommonIEsProvideCapabilities `per:",optional"`
	AGNSSProvideCapabilities     *AGNSSProvideCapabilities     `per:",optional"`
	OTDOAProvideCapabilities     *OTDOAProvideCapabilities     `per:",optional"`
	ECIDProvideCapabilities      *ECIDProvideCapabilities      `per:",optional"`
	EPDUProvideCapabilities      *EPDUSequence                 `per:",optional"`
}

type RequestAssistanceData struct {
	CriticalExtensions RequestAssistanceDataCriticalExtensions
}

type RequestAssistanceDataCriticalExtensions struct {
	C1                       *RequestAssistanceDataCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                  `per:",choice:1,optional"`
}

type RequestAssistanceDataCriticalExtensionsC1 struct {
	RequestAssistanceDataR9 *RequestAssistanceDataR9IEs `per:",choice:0,optional"`
	Spare3                  *per.Null                   `per:",choice:1,optional"`
	Spare2                  *per.Null                   `per:",choice:2,optional"`
	Spare1                  *per.Null                   `per:",choice:3,optional"`
}

type RequestAssistanceDataR9IEs struct {
	_                              [0]struct{}                     `per:"extseq"`
	CommonIEsRequestAssistanceData *CommonIEsRequestAssistanceData `per:",optional"`
	AGNSSRequestAssistanceData     *AGNSSRequestAssistanceData     `per:",optional"`
	OTDOARequestAssistanceData     *OTDOARequestAssistanceData     `per:",optional"`
	EPDURequestAssistanceData      *EPDUSequence                   `per:",optional"`
}

type ProvideAssistanceData struct {
	CriticalExtensions ProvideAssistanceDataCriticalExtensions
}

type ProvideAssistanceDataCriticalExtensions struct {
	C1                       *ProvideAssistanceDataCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                  `per:",choice:1,optional"`
}

type ProvideAssistanceDataCriticalExtensionsC1 struct {
	ProvideAssistanceDataR9 *ProvideAssistanceDataR9IEs `per:",choice:0,optional"`
	Spare3                  *per.Null                   `per:",choice:1,optional"`
	Spare2                  *per.Null                   `per:",choice:2,optional"`
	Spare1                  *per.Null                   `per:",choice:3,optional"`
}

type ProvideAssistanceDataR9IEs struct {
	_                              [0]struct{}                     `per:"extseq"`
	CommonIEsProvideAssistanceData *CommonIEsProvideAssistanceData `per:",optional"`
	AGNSSProvideAssistanceData     *AGNSSProvideAssistanceData     `per:",optional"`
	OTDOAProvideAssistanceData     *OTDOAProvideAssistanceData     `per:",optional"`
	EPDUProvideAssistanceData      *EPDUSequence                   `per:",optional"`
}

type RequestLocationInformation struct {
	CriticalExtensions RequestLocationInformationCriticalExtensions
}

type RequestLocationInformationCriticalExtensions struct {
	C1                       *RequestLocationInformationCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                       `per:",choice:1,optional"`
}

type RequestLocationInformationCriticalExtensionsC1 struct {
	RequestLocationInformationR9 *RequestLocationInformationR9IEs `per:",choice:0,optional"`
	Spare3                       *per.Null                        `per:",choice:1,optional"`
	Spare2                       *per.Null                        `per:",choice:2,optional"`
	Spare1                       *per.Null                        `per:",choice:3,optional"`
}

type RequestLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsRequestLocationInformation *CommonIEsRequestLocationInformation `per:",optional"`
	AGNSSRequestLocationInformation     *AGNSSRequestLocationInformation     `per:",optional"`
	OTDOARequestLocationInformation     *OTDOARequestLocationInformation     `per:",optional"`
	ECIDRequestLocationInformation      *ECIDRequestLocationInformation      `per:",optional"`
	EPDURequestLocationInformation      *EPDUSequence                        `per:",optional"`
}

type ProvideLocationInformation struct {
	CriticalExtensions ProvideLocationInformationCriticalExtensions
}

type ProvideLocationInformationCriticalExtensions struct {
	C1                       *ProvideLocationInformationCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                       `per:",choice:1,optional"`
}

type ProvideLocationInformationCriticalExtensionsC1 struct {
	ProvideLocationInformationR9 *ProvideLocationInformationR9IEs `per:",choice:0,optional"`
	Spare3                       *per.Null                        `per:",choice:1,optional"`
	Spare2                       *per.Null                        `per:",choice:2,optional"`
	Spare1                       *per.Null                        `per:",choice:3,optional"`
}

type ProvideLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsProvideLocationInformation *CommonIEsProvideLocationInformation `per:",optional"`
	AGNSSProvideLocationInformation     *AGNSSProvideLocationInformation     `per:",optional"`
	OTDOAProvideLocationInformation     *OTDOAProvideLocationInformation     `per:",optional"`
	ECIDProvideLocationInformation      *ECIDProvideLocationInformation      `per:",optional"`
	EPDUProvideLocationInformation      *EPDUSequence                        `per:",optional"`
}

type Abort struct {
	CriticalExtensions AbortCriticalExtensions
}

type AbortCriticalExtensions struct {
	C1                       *AbortCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                  `per:",choice:1,optional"`
}

type AbortCriticalExtensionsC1 struct {
	AbortR9 *AbortR9IEs `per:",choice:0,optional"`
	Spare3  *per.Null   `per:",choice:1,optional"`
	Spare2  *per.Null   `per:",choice:2,optional"`
	Spare1  *per.Null   `per:",choice:3,optional"`
}

type AbortR9IEs struct {
	_              [0]struct{}     `per:"extseq"`
	CommonIEsAbort *CommonIEsAbort `per:",optional"`
}

const (
	AbortCauseUndefined                          int64 = 0
	AbortCauseStopPeriodicReporting              int64 = 1
	AbortCauseTargetDeviceAbort                  int64 = 2
	AbortCauseNetworkAbort                       int64 = 3
	AbortCauseStopPeriodicAssistanceDataDelivery int64 = 4
)

type CommonIEsAbort struct {
	AbortCause int64 `per:"ENUMERATED,range:0..3,..."`
}

type Error struct {
	ErrorR9                  *ErrorR9IEs `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null   `per:",choice:1,optional"`
}

type ErrorR9IEs struct {
	_              [0]struct{}     `per:"extseq"`
	CommonIEsError *CommonIEsError `per:",optional"`
}

const (
	ErrorCauseUndefined             int64 = 0
	ErrorCauseLPPMessageHeaderError int64 = 1
	ErrorCauseLPPMessageBodyError   int64 = 2
	ErrorCauseEPDUError             int64 = 3
	ErrorCauseIncorrectDataValue    int64 = 4
	ErrorCauseLPPSegmentationError  int64 = 5
)

type CommonIEsError struct {
	ErrorCause int64 `per:"ENUMERATED,range:0..4,..."`
}

type EPDUSequence struct {
	List []EPDU `per:"SEQUENCE-OF,size:1..16"`
}

type EPDU struct {
	EPDUIdentifier EPDUIdentifier
	EPDUBody       []byte
}

type EPDUIdentifier struct {
	_        [0]struct{} `per:"extseq"`
	EPDUID   int64       `per:",range:1..256"`
	EPDUName *string     `per:"VisibleString,optional,size:1..32"`
}
