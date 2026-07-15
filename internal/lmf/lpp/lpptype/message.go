// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lpptype holds hand-written Go structs that mirror the 3GPP TS 37.355
// (Rel-18) LPP ASN.1 definitions needed for A-GNSS positioning.
//
// LPP is carried in the unaligned variant of PER (TS 37.355 §5), so these
// structs are read and written by the hand-written codec in the parent package,
// not by a reflection codec. The `aper` tags are inert: no encoder consults
// them, and where they disagree with the ASN.1 the codec is what governs. Only
// the aper.Enumerated and aper.BitString field types are load-bearing.
//
// A struct here is not evidence of what the spec says. Several were modelled
// with fewer root fields than TS 37.355 defines, which under PER shifts every
// bit that follows, so check the ASN.1 before trusting a shape.
package lpptype

import (
	"fmt"

	"github.com/free5gc/aper"
)

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

//	CommonIEsAbort ::= SEQUENCE {
//	    abortCause ENUMERATED { undefined, stopPeriodicReporting, targetDeviceAbort,
//	        networkAbort, ..., stopPeriodicAssistanceDataDelivery-v1510 } }
const (
	CommonIEsAbortCausePresentUndefined             aper.Enumerated = 0
	CommonIEsAbortCausePresentStopPeriodicReporting aper.Enumerated = 1
	CommonIEsAbortCausePresentTargetDeviceAbort     aper.Enumerated = 2
	CommonIEsAbortCausePresentNetworkAbort          aper.Enumerated = 3
)

// AbortCauseString names an abortCause for logging. An abort is the only
// account a target gives of why it walked away, so the cause is reported
// verbatim rather than folded into a generic failure.
func AbortCauseString(c aper.Enumerated) string {
	switch c {
	case CommonIEsAbortCausePresentUndefined:
		return "undefined"
	case CommonIEsAbortCausePresentStopPeriodicReporting:
		return "stopPeriodicReporting"
	case CommonIEsAbortCausePresentTargetDeviceAbort:
		return "targetDeviceAbort"
	case CommonIEsAbortCausePresentNetworkAbort:
		return "networkAbort"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

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
