// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lpptype holds hand-written, aper-tagged Go structs that mirror the
// 3GPP TS 37.355 (Rel-18) LPP ASN.1 definitions needed for A-GNSS positioning.
//
// The aligned-PER codec is github.com/free5gc/aper. Tag conventions follow the
// existing internal/nrppa/nrppatype package:
//   - SEQUENCE: Go struct, optional fields tagged `aper:"optional"`
//   - CHOICE: Go struct with Present int + pointer fields; referencing field
//     tagged `aper:"valueLB:0,valueUB:N-1"` (N = number of root alternatives)
//   - Extensible types: referencing field tagged with `valueExt`
//   - ENUMERATED: aper.Enumerated with `valueLB:0,valueUB:N-1,valueExt`
//   - BIT STRING: aper.BitString with `sizeLB:N,sizeUB:N`
//   - INTEGER: int64 with `valueLB:Lo,valueUB:Hi`
package lpptype

import "github.com/free5gc/aper"

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
	TransactionID   *LPPTransactionID `aper:"optional,valueExt"`
	EndTransaction  bool
	SequenceNumber  *int64           `aper:"optional,valueLB:0,valueUB:255"`
	Acknowledgement *Acknowledgement `aper:"optional"`
	LppMessageBody  *LPPMessageBody  `aper:"optional,valueLB:0,valueUB:1"`
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
	Initiator         Initiator
	TransactionNumber int64 `aper:"valueLB:0,valueUB:255"`
}

// Initiator ::= ENUMERATED { locationServer, targetDevice, ... }
const (
	InitiatorLocationServer aper.Enumerated = 0
	InitiatorTargetDevice   aper.Enumerated = 1
)

type Initiator struct {
	Value aper.Enumerated `aper:"valueLB:0,valueUB:1,valueExt"`
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
	AckIndicator *int64 `aper:"optional,valueLB:0,valueUB:255"`
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
// Outer CHOICE: 2 alternatives, not extensible → valueLB:0, valueUB:1.
type LPPMessageBody struct {
	Present               int
	C1                    *LPPMessageBodyC1 `aper:"valueLB:0,valueUB:15"`
	MessageClassExtension *struct{}
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
	Present                    int
	RequestCapabilities        *RequestCapabilities
	ProvideCapabilities        *ProvideCapabilities
	RequestAssistanceData      *RequestAssistanceData
	ProvideAssistanceData      *ProvideAssistanceData
	RequestLocationInformation *RequestLocationInformation
	ProvideLocationInformation *ProvideLocationInformation
	Abort                      *Abort
	Error                      *Error
	Spare7                     *struct{}
	Spare6                     *struct{}
	Spare5                     *struct{}
	Spare4                     *struct{}
	Spare3                     *struct{}
	Spare2                     *struct{}
	Spare1                     *struct{}
	Spare0                     *struct{}
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
	CriticalExtensions AbortCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type AbortCriticalExtensions struct {
	Present                  int
	C1                       *AbortCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type AbortCriticalExtensionsC1 struct {
	Present int
	AbortR9 *AbortR9IEs
	Spare3  *struct{}
	Spare2  *struct{}
	Spare1  *struct{}
}

type AbortR9IEs struct {
	CommonIEsAbort *CommonIEsAbort `aper:"optional"`
}

// CommonIEsAbort ::= SEQUENCE { abortCause ENUMERATED {...}, ... }
const (
	CommonIEsAbortCausePresentUndefined        aper.Enumerated = 0
	CommonIEsAbortCausePresentStopPeriodicEcid aper.Enumerated = 1
)

type CommonIEsAbort struct {
	AbortCause struct {
		Value aper.Enumerated `aper:"valueLB:0,valueUB:3,valueExt"`
	}
}

//	Error ::= CHOICE {
//	    error-r9     Error-r9-IEs,
//	    criticalExtensionsFuture SEQUENCE {}
//	}
type Error struct {
	Present                  int
	ErrorR9                  *ErrorR9IEs
	CriticalExtensionsFuture *struct{}
}

type ErrorR9IEs struct {
	CommonIEsError *CommonIEsError `aper:"optional"`
}

// CommonIEsError ::= SEQUENCE { errorCause ENUMERATED {...}, ... }
const (
	CommonIEsErrorErrorCausePresentUndefined             aper.Enumerated = 0
	CommonIEsErrorErrorCausePresentLPPMessageHeaderError aper.Enumerated = 1
	CommonIEsErrorErrorCausePresentLPPMessageBodyError   aper.Enumerated = 2
	CommonIEsErrorErrorCausePresentEPDUError             aper.Enumerated = 3
	CommonIEsErrorErrorCausePresentIncorrectDataValue    aper.Enumerated = 4
)

type CommonIEsError struct {
	ErrorCause struct {
		Value aper.Enumerated `aper:"valueLB:0,valueUB:4,valueExt"`
	}
}
