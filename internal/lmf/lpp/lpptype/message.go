// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lpptype holds hand-written, per-tagged Go structs that mirror the
// 3GPP TS 37.355 (Rel-18) LPP ASN.1 definitions needed for A-GNSS positioning.
//
// The PER codec is github.com/ellanetworks/core/internal/per using the Unaligned
// variant (BASIC-PER Unaligned, per TS 37.355 §7). Tag conventions:
//   - SEQUENCE: Go struct, optional fields tagged `per:",optional"`
//   - CHOICE: Go struct with pointer fields tagged `per:",choice:N,optional"`
//   - Extensible types: placeholder field tagged `per:"extseq"`
//   - ENUMERATED: int64 with `per:",range:0..N"`
//   - BIT STRING: []bool with `per:",size:lb..ub"`
//   - INTEGER: int64 with `per:",range:lb..ub"`
package lpptype

import "github.com/ellanetworks/core/internal/per"

//go:generate go run github.com/ellanetworks/core/cmd/pergen

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
	LppMessageBody  *LPPMessageBody  `per:",optional"`
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
	Initiator         Initiator
	TransactionNumber int64 `per:",range:0..255"`
}

// Initiator ::= ENUMERATED { locationServer, targetDevice, ... }
const (
	InitiatorLocationServer int64 = 0
	InitiatorTargetDevice   int64 = 1
)

type Initiator struct {
	Value int64 `per:",range:0..1,..."`
}

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
	CommonIEsAbort *CommonIEsAbort `per:",optional"`
}

// CommonIEsAbort ::= SEQUENCE { abortCause ENUMERATED {...}, ... }
const (
	CommonIEsAbortCausePresentUndefined        int64 = 0
	CommonIEsAbortCausePresentStopPeriodicEcid int64 = 1
)

type CommonIEsAbort struct {
	_          [0]struct{} `per:"extseq"`
	AbortCause int64       `per:",range:0..3,..."`
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
	CommonIEsError *CommonIEsError `per:",optional"`
}

// CommonIEsError ::= SEQUENCE { errorCause ENUMERATED {...}, ... }
const (
	CommonIEsErrorErrorCausePresentUndefined             int64 = 0
	CommonIEsErrorErrorCausePresentLPPMessageHeaderError int64 = 1
	CommonIEsErrorErrorCausePresentLPPMessageBodyError   int64 = 2
	CommonIEsErrorErrorCausePresentEPDUError             int64 = 3
	CommonIEsErrorErrorCausePresentIncorrectDataValue    int64 = 4
)

type CommonIEsError struct {
	_          [0]struct{} `per:"extseq"`
	ErrorCause int64       `per:",range:0..4,..."`
}
