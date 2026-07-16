// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
)

// enumValue adapts a decoded root index to the aper.Enumerated the lpptype
// model stores. The model keeps free5gc's type so callers are unchanged while
// the codec moves to the unaligned reader/writer.
func enumValue(index int) aper.Enumerated {
	return aper.Enumerated(index)
}

// aperEnum aliases the enumerated type the lpptype model stores.
type aperEnum = aper.Enumerated

// bitString adapts a decoded bit string to the model's representation.
func bitString(b []byte, nbits int) aper.BitString {
	return aper.BitString{Bytes: b, BitLength: nbits}
}

// This file hand-encodes LPP against the UNALIGNED variant of X.691, which
// TS 37.355 §5 mandates ("BASIC-PER, Unaligned Variant"). The ALIGNED variant
// the other 3GPP protocols in this repo use (S1AP/NGAP/NRPPa) pads productions
// to octet boundaries that an LPP peer does not expect: a handset answers such a
// message with an LPP Error, and its own reply fails to decode.
//
// Root alternative counts for the CHOICEs encoded here (TS 37.355 §6.2).
const (
	nRootMessageBody     = 2  // c1, messageClassExtension
	nRootMessageBodyC1   = 16 // 8 messages + 8 spares
	nRootCriticalExt     = 2  // c1, criticalExtensionsFuture
	nRootCriticalExtC1   = 4  // <msg>-r9, spare3, spare2, spare1
	nRootErrorChoice     = 2  // error-r9, criticalExtensionsFuture
	nRootInitiator       = 2  // locationServer, targetDevice
	nRootErrorCause      = 5  // undefined..incorrectDataValue
	transactionNumberMin = 0
	transactionNumberMax = 255
	sequenceNumberMin    = 0
	sequenceNumberMax    = 255
)

// EncodeMessage serialises an LPP-Message using unaligned PER.
//
//	LPP-Message ::= SEQUENCE {
//	    transactionID   LPP-TransactionID OPTIONAL,
//	    endTransaction  BOOLEAN,
//	    sequenceNumber  SequenceNumber    OPTIONAL,
//	    acknowledgement Acknowledgement   OPTIONAL,
//	    lpp-MessageBody LPP-MessageBody   OPTIONAL }
func EncodeMessage(msg *lpptype.LPPMessage) ([]byte, error) {
	w := aper.NewUnalignedWriter()

	w.WriteSequencePreamble(false, false, []bool{
		msg.TransactionID != nil,
		msg.SequenceNumber != nil,
		msg.Acknowledgement != nil,
		msg.LppMessageBody != nil,
	})

	if msg.TransactionID != nil {
		if err := writeTransactionID(w, msg.TransactionID); err != nil {
			return nil, err
		}
	}

	w.WriteBool(msg.EndTransaction)

	if msg.SequenceNumber != nil {
		if err := w.WriteConstrainedInt(*msg.SequenceNumber, sequenceNumberMin, sequenceNumberMax); err != nil {
			return nil, fmt.Errorf("sequenceNumber: %w", err)
		}
	}

	if msg.Acknowledgement != nil {
		if err := writeAcknowledgement(w, msg.Acknowledgement); err != nil {
			return nil, err
		}
	}

	if msg.LppMessageBody != nil {
		if err := writeMessageBody(w, msg.LppMessageBody); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// DecodeMessage parses an LPP-Message from unaligned PER.
func DecodeMessage(data []byte) (*lpptype.LPPMessage, error) {
	r := aper.NewUnalignedReader(data)

	msg, optionals, err := readEnvelopeHeader(r)
	if err != nil {
		return nil, err
	}

	if optionals[3] {
		if msg.LppMessageBody, err = readMessageBody(r); err != nil {
			return nil, err
		}
	}

	return msg, nil
}

// DecodeEnvelopeHeader parses only the fields that precede lpp-MessageBody:
// transactionID, endTransaction, sequenceNumber and acknowledgement. TS 37.355
// §4.3.4 requires an acknowledgement whenever ackRequested and the sequence
// number can be read, regardless of whether the body decodes, so those fields
// are recoverable from a PDU whose body is malformed.
func DecodeEnvelopeHeader(data []byte) (*lpptype.LPPMessage, error) {
	msg, _, err := readEnvelopeHeader(aper.NewUnalignedReader(data))

	return msg, err
}

// readEnvelopeHeader reads the LPP-Message fields up to and including
// acknowledgement, returning the partially populated message and the SEQUENCE
// preamble so the caller can decide whether a body follows.
func readEnvelopeHeader(r *aper.Reader) (*lpptype.LPPMessage, []bool, error) {
	_, optionals, err := r.ReadSequencePreamble(false, 4)
	if err != nil {
		return nil, nil, fmt.Errorf("LPP-Message preamble: %w", err)
	}

	msg := &lpptype.LPPMessage{}

	if optionals[0] {
		if msg.TransactionID, err = readTransactionID(r); err != nil {
			return nil, nil, err
		}
	}

	if msg.EndTransaction, err = r.ReadBool(); err != nil {
		return nil, nil, fmt.Errorf("endTransaction: %w", err)
	}

	if optionals[1] {
		seq, err := r.ReadConstrainedInt(sequenceNumberMin, sequenceNumberMax)
		if err != nil {
			return nil, nil, fmt.Errorf("sequenceNumber: %w", err)
		}

		msg.SequenceNumber = &seq
	}

	if optionals[2] {
		if msg.Acknowledgement, err = readAcknowledgement(r); err != nil {
			return nil, nil, err
		}
	}

	return msg, optionals, nil
}

// LPP-TransactionID ::= SEQUENCE { initiator Initiator, transactionNumber TransactionNumber, ... }
func writeTransactionID(w *aper.Writer, id *lpptype.LPPTransactionID) error {
	w.WriteSequencePreamble(true, false, nil)

	if err := w.WriteEnum(int(id.Initiator.Value), nRootInitiator, true, false); err != nil {
		return fmt.Errorf("initiator: %w", err)
	}

	if err := w.WriteConstrainedInt(id.TransactionNumber, transactionNumberMin, transactionNumberMax); err != nil {
		return fmt.Errorf("transactionNumber: %w", err)
	}

	return nil
}

func readTransactionID(r *aper.Reader) (*lpptype.LPPTransactionID, error) {
	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("LPP-TransactionID preamble: %w", err)
	}

	initiator, isExt, err := r.ReadEnum(nRootInitiator, true)
	if err != nil {
		return nil, fmt.Errorf("initiator: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("initiator: unsupported extension value")
	}

	num, err := r.ReadConstrainedInt(transactionNumberMin, transactionNumberMax)
	if err != nil {
		return nil, fmt.Errorf("transactionNumber: %w", err)
	}

	id := &lpptype.LPPTransactionID{TransactionNumber: num}
	id.Initiator.Value = enumValue(initiator)

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, fmt.Errorf("LPP-TransactionID extensions: %w", err)
		}
	}

	return id, nil
}

// Acknowledgement ::= SEQUENCE { ackRequested BOOLEAN, ackIndicator SequenceNumber OPTIONAL }
func writeAcknowledgement(w *aper.Writer, ack *lpptype.Acknowledgement) error {
	w.WriteSequencePreamble(false, false, []bool{ack.AckIndicator != nil})
	w.WriteBool(ack.AckRequested)

	if ack.AckIndicator != nil {
		if err := w.WriteConstrainedInt(*ack.AckIndicator, sequenceNumberMin, sequenceNumberMax); err != nil {
			return fmt.Errorf("ackIndicator: %w", err)
		}
	}

	return nil
}

func readAcknowledgement(r *aper.Reader) (*lpptype.Acknowledgement, error) {
	_, optionals, err := r.ReadSequencePreamble(false, 1)
	if err != nil {
		return nil, fmt.Errorf("acknowledgement preamble: %w", err)
	}

	ack := &lpptype.Acknowledgement{}

	if ack.AckRequested, err = r.ReadBool(); err != nil {
		return nil, fmt.Errorf("ackRequested: %w", err)
	}

	if optionals[0] {
		ind, err := r.ReadConstrainedInt(sequenceNumberMin, sequenceNumberMax)
		if err != nil {
			return nil, fmt.Errorf("ackIndicator: %w", err)
		}

		ack.AckIndicator = &ind
	}

	return ack, nil
}

// writeMessageBody encodes LPP-MessageBody. Only the bodies the LMF originates
// are supported; the rest report an error rather than emit a wrong encoding.
func writeMessageBody(w *aper.Writer, body *lpptype.LPPMessageBody) error {
	if body.Present != 1 || body.C1 == nil {
		return fmt.Errorf("lpp-MessageBody: only the c1 alternative is supported")
	}

	if err := w.WriteChoiceIndex(0, nRootMessageBody, false, false); err != nil {
		return fmt.Errorf("lpp-MessageBody choice: %w", err)
	}

	// The Present constants are 1-based (Nothing = 0), the choice index is not.
	if err := w.WriteChoiceIndex(body.C1.Present-1, nRootMessageBodyC1, false, false); err != nil {
		return fmt.Errorf("lpp-MessageBody c1 choice: %w", err)
	}

	switch body.C1.Present {
	case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
		return writeRequestCapabilities(w, body.C1.RequestCapabilities)
	case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
		return writeRequestLocationInformation(w, body.C1.RequestLocationInformation)
	case lpptype.LPPMessageBodyC1PresentProvideCapabilities:
		return writeProvideCapabilities(w, body.C1.ProvideCapabilities)
	case lpptype.LPPMessageBodyC1PresentProvideLocationInformation:
		return writeProvideLocationInformation(w, body.C1.ProvideLocationInformation)
	default:
		return fmt.Errorf("lpp-MessageBody: encoding c1 alternative %d is not implemented", body.C1.Present-1)
	}
}

func readMessageBody(r *aper.Reader) (*lpptype.LPPMessageBody, error) {
	choice, _, err := r.ReadChoiceIndex(nRootMessageBody, false)
	if err != nil {
		return nil, fmt.Errorf("lpp-MessageBody choice: %w", err)
	}

	if choice != 0 {
		return nil, fmt.Errorf("lpp-MessageBody: messageClassExtension is not supported")
	}

	c1, _, err := r.ReadChoiceIndex(nRootMessageBodyC1, false)
	if err != nil {
		return nil, fmt.Errorf("lpp-MessageBody c1 choice: %w", err)
	}

	body := &lpptype.LPPMessageBody{Present: 1, C1: &lpptype.LPPMessageBodyC1{Present: c1 + 1}}

	switch body.C1.Present {
	case lpptype.LPPMessageBodyC1PresentProvideCapabilities:
		if body.C1.ProvideCapabilities, err = readProvideCapabilities(r); err != nil {
			return nil, err
		}
	case lpptype.LPPMessageBodyC1PresentProvideLocationInformation:
		if body.C1.ProvideLocationInformation, err = readProvideLocationInformation(r); err != nil {
			return nil, err
		}
	case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
		if body.C1.RequestCapabilities, err = readRequestCapabilities(r); err != nil {
			return nil, err
		}
	case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
		if body.C1.RequestLocationInformation, err = readRequestLocationInformation(r); err != nil {
			return nil, err
		}
	case lpptype.LPPMessageBodyC1PresentProvideAssistanceData:
		if body.C1.ProvideAssistanceData, err = readProvideAssistanceData(r); err != nil {
			return nil, err
		}
	case lpptype.LPPMessageBodyC1PresentError:
		if body.C1.Error, err = readError(r); err != nil {
			return nil, err
		}
	case lpptype.LPPMessageBodyC1PresentAbort:
		if body.C1.Abort, err = readAbort(r); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("lpp-MessageBody: decoding c1 alternative %d is not implemented", c1)
	}

	return body, nil
}

//	RequestCapabilities ::= SEQUENCE {
//	    criticalExtensions CHOICE {
//	        c1 CHOICE { requestCapabilities-r9 RequestCapabilities-r9-IEs, spare3, spare2, spare1 },
//	        criticalExtensionsFuture SEQUENCE {} } }
func writeRequestCapabilities(w *aper.Writer, req *lpptype.RequestCapabilities) error {
	if req == nil || req.CriticalExtensions.Present != 1 || req.CriticalExtensions.C1 == nil {
		return fmt.Errorf("RequestCapabilities: only the c1 critical extension is supported")
	}

	if err := w.WriteChoiceIndex(0, nRootCriticalExt, false, false); err != nil {
		return fmt.Errorf("RequestCapabilities criticalExtensions: %w", err)
	}

	if err := w.WriteChoiceIndex(0, nRootCriticalExtC1, false, false); err != nil {
		return fmt.Errorf("RequestCapabilities c1: %w", err)
	}

	ies := req.CriticalExtensions.C1.RequestCapabilitiesR9
	if ies == nil {
		return fmt.Errorf("RequestCapabilities: requestCapabilities-r9 is required")
	}

	//	RequestCapabilities-r9-IEs ::= SEQUENCE {
	//	    commonIEsRequestCapabilities OPTIONAL, a-gnss-RequestCapabilities OPTIONAL,
	//	    otdoa-RequestCapabilities OPTIONAL, ecid-RequestCapabilities OPTIONAL,
	//	    epdu-RequestCapabilities OPTIONAL, ... }
	w.WriteSequencePreamble(true, false, []bool{
		ies.CommonIEsRequestCapabilities != nil,
		ies.AGNSSRequestCapabilities != nil,
		ies.OTDOARequestCapabilities != nil,
		ies.ECIDRequestCapabilities != nil,
		ies.EPDURequestCapabilities != nil,
	})

	if ies.AGNSSRequestCapabilities != nil {
		//	A-GNSS-RequestCapabilities ::= SEQUENCE {
		//	    gnss-SupportListReq BOOLEAN, assistanceDataSupportListReq BOOLEAN,
		//	    locationVelocityTypesReq BOOLEAN, ... }
		w.WriteSequencePreamble(true, false, nil)
		w.WriteBool(ies.AGNSSRequestCapabilities.GnssSupportListReq)
		w.WriteBool(ies.AGNSSRequestCapabilities.AssistanceDataSupportListReq)
		w.WriteBool(ies.AGNSSRequestCapabilities.LocationVelocityTypesReq)
	}

	return nil
}

// Error ::= CHOICE { error-r9 Error-r9-IEs, criticalExtensionsFuture SEQUENCE {} }
func readError(r *aper.Reader) (*lpptype.Error, error) {
	choice, _, err := r.ReadChoiceIndex(nRootErrorChoice, false)
	if err != nil {
		return nil, fmt.Errorf("error choice: %w", err)
	}

	out := &lpptype.Error{Present: choice + 1}

	if choice != 0 {
		return out, nil // criticalExtensionsFuture carries an empty SEQUENCE
	}

	//	Error-r9-IEs ::= SEQUENCE { commonIEsError CommonIEsError OPTIONAL, ..., epdu-Error OPTIONAL }
	extPresent, optionals, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return nil, fmt.Errorf("Error-r9-IEs preamble: %w", err)
	}

	ies := &lpptype.ErrorR9IEs{}

	if optionals[0] {
		//	CommonIEsError ::= SEQUENCE { errorCause ENUMERATED {...} }
		if _, _, err := r.ReadSequencePreamble(false, 0); err != nil {
			return nil, fmt.Errorf("CommonIEsError preamble: %w", err)
		}

		cause, isExt, err := r.ReadEnum(nRootErrorCause, true)
		if err != nil {
			return nil, fmt.Errorf("errorCause: %w", err)
		}

		ies.CommonIEsError = &lpptype.CommonIEsError{}
		if !isExt {
			ies.CommonIEsError.ErrorCause.Value = enumValue(cause)
		}
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, fmt.Errorf("Error-r9-IEs extensions: %w", err)
		}
	}

	out.ErrorR9 = ies

	return out, nil
}
