// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lpptype holds hand-written, per-tagged Go structs that mirror the
// 3GPP TS 37.355 (Rel-18) LPP ASN.1 definitions reachable from LPP-Message.
//
// The PER codec is github.com/ellanetworks/core/per using the Unaligned
// variant (BASIC-PER Unaligned, per TS 37.355 §7). Tag conventions:
//   - SEQUENCE: Go struct, optional fields tagged `per:",optional"`
//   - CHOICE: Go struct with pointer fields tagged `per:",choice:N,optional"`
//   - Extensible types: placeholder field tagged `per:"extseq"`
//   - ENUMERATED: named integer type with `per:"ENUMERATED,range:0..N"`
//   - BIT STRING: []bool with `per:",size:lb..ub"`
//   - INTEGER: int64 with `per:",range:lb..ub"`
package lpptype

import "github.com/ellanetworks/core/per"

//go:generate sh -c "rm -f per_gen.go && go run github.com/ellanetworks/core/cmd/pergen"

// =====================================================================
// LPP-Message (TS 37.355 §6.2)
// =====================================================================

//	LPP-Message ::= SEQUENCE {
//	    transactionID   LPP-TransactionID OPTIONAL, -- Need ON
//	    endTransaction   BOOLEAN,
//	    sequenceNumber   SequenceNumber  OPTIONAL, -- Need ON
//	    acknowledgement   Acknowledgement  OPTIONAL, -- Need ON
//	    lpp-MessageBody   LPP-MessageBody  OPTIONAL -- Need ON
//	}
//
// Not extensible (no "..." in the spec). 4 optional fields.
type LPPMessage struct {
	TransactionID   *LPPTransactionID `per:",optional"`
	EndTransaction  bool
	SequenceNumber  *int64           `per:",optional,range:0..255"`
	Acknowledgement *Acknowledgement `per:",optional"`
	LPPMessageBody  *LPPMessageBody  `per:",optional"`
}

// =====================================================================
// LPP-TransactionID (TS 37.355 §6.2)
// =====================================================================

//	LPP-TransactionID ::= SEQUENCE {
//	    initiator        Initiator,
//	    transactionNumber  TransactionNumber,
//	    ...
//	}
//
// Extensible SEQUENCE with 2 mandatory fields.
type LPPTransactionID struct {
	_                 [0]struct{} `per:"extseq"`
	Initiator         Initiator   `per:"ENUMERATED,range:0..1,...,extvalues:0"`
	TransactionNumber int64       `per:",range:0..255"`
}

// Initiator ::= ENUMERATED { locationServer, targetDevice, ... }
type Initiator int64

const (
	InitiatorLocationServer Initiator = 0
	InitiatorTargetDevice   Initiator = 1
)

// =====================================================================
// Acknowledgement (TS 37.355 §6.2)
// =====================================================================

//	Acknowledgement ::= SEQUENCE {
//	    ackRequested BOOLEAN,
//	    ackIndicator SequenceNumber  OPTIONAL
//	}
type Acknowledgement struct {
	AckRequested bool
	AckIndicator *int64 `per:",optional,range:0..255"`
}

// =====================================================================
// LPP-MessageBody (TS 37.355 §6.2)
// =====================================================================

//	LPP-MessageBody ::= CHOICE {
//	    c1      CHOICE {
//	        requestCapabilities   RequestCapabilities,
//	        provideCapabilities   ProvideCapabilities,
//	        requestAssistanceData  RequestAssistanceData,
//	        provideAssistanceData  ProvideAssistanceData,
//	        requestLocationInformation RequestLocationInformation,
//	        provideLocationInformation ProvideLocationInformation,
//	        abort      Abort,
//	        error      Error,
//	        spare7 NULL, spare6 NULL, spare5 NULL, spare4 NULL,
//	        spare3 NULL, spare2 NULL, spare1 NULL, spare0 NULL
//	    },
//	    messageClassExtension SEQUENCE {}
//	}
//
// Outer CHOICE: 2 alternatives, not extensible.
type LPPMessageBody struct {
	C1                    *LPPMessageBodyC1 `per:",choice:0,optional"`
	MessageClassExtension *per.Null         `per:",choice:1,optional"`
}

// Inner c1 CHOICE: 16 alternatives (8 root + 8 spare NULL), not extensible.
// All 16 must be modelled so the choice index uses 4 bits (ceil(log2(16))).
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
	LPPMessageBodyC1PresentSpare7
	LPPMessageBodyC1PresentSpare6
	LPPMessageBodyC1PresentSpare5
	LPPMessageBodyC1PresentSpare4
	LPPMessageBodyC1PresentSpare3
	LPPMessageBodyC1PresentSpare2
	LPPMessageBodyC1PresentSpare1
	LPPMessageBodyC1PresentSpare0
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

// =====================================================================
// RequestCapabilities (TS 37.355 §6.3)
// =====================================================================

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

// RequestCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type RequestCapabilitiesR9IEs struct {
	_                            [0]struct{}                   `per:"extseq"`
	CommonIEsRequestCapabilities *CommonIEsRequestCapabilities `per:",optional"`
	AGNSSRequestCapabilities     *AGNSSRequestCapabilities     `per:",optional"`
	OTDOARequestCapabilities     *OTDOARequestCapabilities     `per:",optional"`
	ECIDRequestCapabilities      *ECIDRequestCapabilities      `per:",optional"`
	EPDURequestCapabilities      *EPDUSequence                 `per:",optional"`
}

// =====================================================================
// ProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

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

// ProvideCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type ProvideCapabilitiesR9IEs struct {
	_                            [0]struct{}                   `per:"extseq"`
	CommonIEsProvideCapabilities *CommonIEsProvideCapabilities `per:",optional"`
	AGNSSProvideCapabilities     *AGNSSProvideCapabilities     `per:",optional"`
	OTDOAProvideCapabilities     *OTDOAProvideCapabilities     `per:",optional"`
	ECIDProvideCapabilities      *ECIDProvideCapabilities      `per:",optional"`
	EPDUProvideCapabilities      *EPDUSequence                 `per:",optional"`
}

// =====================================================================
// RequestAssistanceData (TS 37.355 §6.3)
// =====================================================================

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

// RequestAssistanceData-r9-IEs: extensible SEQUENCE.
// TS 37.355 §6.3 line 1147-1156.
type RequestAssistanceDataR9IEs struct {
	_                              [0]struct{}                     `per:"extseq"`
	CommonIEsRequestAssistanceData *CommonIEsRequestAssistanceData `per:",optional"`
	AGNSSRequestAssistanceData     *AGNSSRequestAssistanceData     `per:",optional"`
	OTDOARequestAssistanceData     *OTDOARequestAssistanceData     `per:",optional"`
	EPDURequestAssistanceData      *EPDUSequence                   `per:",optional"`
}

// =====================================================================
// ProvideAssistanceData (TS 37.355 §6.3)
// =====================================================================

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

// ProvideAssistanceData-r9-IEs: extensible SEQUENCE.
// TS 37.355 §6.3 line 2693-2707.
type ProvideAssistanceDataR9IEs struct {
	_                              [0]struct{}                     `per:"extseq"`
	CommonIEsProvideAssistanceData *CommonIEsProvideAssistanceData `per:",optional"`
	AGNSSProvideAssistanceData     *AGNSSProvideAssistanceData     `per:",optional"`
	OTDOAProvideAssistanceData     *OTDOAProvideAssistanceData     `per:",optional"`
	EPDUProvideAssistanceData      *EPDUSequence                   `per:",optional"`
}

// =====================================================================
// RequestLocationInformation (TS 37.355 §6.3)
// =====================================================================

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

// RequestLocationInformation-r9-IEs: extensible SEQUENCE with 5 root optional fields.
// TS 37.355 §6.3 line 2781-2793.
type RequestLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsRequestLocationInformation *CommonIEsRequestLocationInformation `per:",optional"`
	AGNSSRequestLocationInformation     *AGNSSRequestLocationInformation     `per:",optional"`
	OTDOARequestLocationInformation     *OTDOARequestLocationInformation     `per:",optional"`
	ECIDRequestLocationInformation      *ECIDRequestLocationInformation      `per:",optional"`
	EPDURequestLocationInformation      *EPDUSequence                        `per:",optional"`
}

// =====================================================================
// ProvideLocationInformation (TS 37.355 §6.3)
// =====================================================================

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

// ProvideLocationInformation-r9-IEs: extensible SEQUENCE.
// TS 37.355 §6.3 line 2935-2943.
type ProvideLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsProvideLocationInformation *CommonIEsProvideLocationInformation `per:",optional"`
	AGNSSProvideLocationInformation     *AGNSSProvideLocationInformation     `per:",optional"`
	OTDOAProvideLocationInformation     *OTDOAProvideLocationInformation     `per:",optional"`
	ECIDProvideLocationInformation      *ECIDProvideLocationInformation      `per:",optional"`
	EPDUProvideLocationInformation      *EPDUSequence                        `per:",optional"`
}

// =====================================================================
// Abort and Error (TS 37.355 §6.3)
// =====================================================================

//	Abort ::= SEQUENCE {
//	    criticalExtensions  CHOICE {
//	        c1      CHOICE {
//	            abort-r9  Abort-r9-IEs,
//	            spare3 NULL, spare2 NULL, spare1 NULL
//	        },
//	        criticalExtensionsFuture SEQUENCE {}
//	    }
//	}
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

// CommonIEsAbort ::= SEQUENCE { abortCause ENUMERATED {...} }
type AbortCause int64

const (
	AbortCauseUndefined                          AbortCause = 0
	AbortCauseStopPeriodicReporting              AbortCause = 1
	AbortCauseTargetDeviceAbort                  AbortCause = 2
	AbortCauseNetworkAbort                       AbortCause = 3
	AbortCauseStopPeriodicAssistanceDataDelivery AbortCause = 4
)

type CommonIEsAbort struct {
	AbortCause AbortCause `per:"ENUMERATED,range:0..3,...,extvalues:1"`
}

//	Error ::= CHOICE {
//	    error-r9     Error-r9-IEs,
//	    criticalExtensionsFuture SEQUENCE {}
//	}
type Error struct {
	ErrorR9                  *ErrorR9IEs `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null   `per:",choice:1,optional"`
}

type ErrorR9IEs struct {
	_              [0]struct{}     `per:"extseq"`
	CommonIEsError *CommonIEsError `per:",optional"`
}

// CommonIEsError ::= SEQUENCE { errorCause ENUMERATED {...} }
type ErrorCause int64

const (
	ErrorCauseUndefined             ErrorCause = 0
	ErrorCauseLPPMessageHeaderError ErrorCause = 1
	ErrorCauseLPPMessageBodyError   ErrorCause = 2
	ErrorCauseEPDUError             ErrorCause = 3
	ErrorCauseIncorrectDataValue    ErrorCause = 4
	ErrorCauseLPPSegmentationError  ErrorCause = 5
)

type CommonIEsError struct {
	ErrorCause ErrorCause `per:"ENUMERATED,range:0..4,...,extvalues:1"`
}

// =====================================================================
// EPDU types (TS 37.355 §6.7)
// =====================================================================

// EPDU-Sequence ::= SEQUENCE (SIZE (1..maxEPDU)) OF EPDU
// maxEPDU INTEGER ::= 16
type EPDUSequence struct {
	List []EPDU `per:"SEQUENCE-OF,size:1..16"`
}

//	EPDU ::= SEQUENCE {
//	    ePDU-Identifier			EPDU-Identifier,
//	    ePDU-Body				EPDU-Body
//	}
type EPDU struct {
	EPDUIdentifier EPDUIdentifier
	EPDUBody       []byte
}

//	EPDU-Identifier ::= SEQUENCE {
//	    ePDU-ID					EPDU-ID,
//	    ePDU-Name				EPDU-Name		OPTIONAL,
//	    ...
//	}
//
// EPDU-ID ::= INTEGER (1..256)
// EPDU-Name ::= VisibleString (SIZE (1..32))
type EPDUIdentifier struct {
	_        [0]struct{} `per:"extseq"`
	EPDUID   int64       `per:",range:1..256"`
	EPDUName *string     `per:"VisibleString,optional,size:1..32"`
}

//	EPDU-Body ::= OCTET STRING
// (encoded as []byte in EPDU struct)
