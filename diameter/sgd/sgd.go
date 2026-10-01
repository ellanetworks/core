// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const ApplicationID uint32 = 16777313

const (
	CommandMOForwardShortMessage uint32 = 8388645
	CommandMTForwardShortMessage uint32 = 8388646
)

const (
	AVPEPSLocationInformation           uint32 = 1496
	AVPMPSPriority                      uint32 = 1616
	AVPNRCellGlobalIdentity             uint32 = 1726
	AVPSMRPUI                           uint32 = 3301
	AVPTFRFlags                         uint32 = 3302
	AVPSMDeliveryFailureCause           uint32 = 3303
	AVPSMEnumeratedDeliveryFailureCause uint32 = 3304
	AVPSMDiagnosticInfo                 uint32 = 3305
	AVPSMDeliveryTimer                  uint32 = 3306
	AVPSMDeliveryStartTime              uint32 = 3307
	AVPOFRFlags                         uint32 = 3328
	AVPMaximumRetransmissionTime        uint32 = 3330
	AVPRequestedRetransmissionTime      uint32 = 3331
	AVPSMSGMSCAddress                   uint32 = 3332
)

const MaxSMRPUILength = 200

const tfrFlagMoreMessagesToSend uint32 = 1 << 0

type DeliveryFailureCause uint32

const (
	CauseMemoryCapacityExceeded DeliveryFailureCause = 0
	CauseEquipmentProtocolError DeliveryFailureCause = 1
	CauseEquipmentNotSMEquipped DeliveryFailureCause = 2
	CauseUnknownServiceCentre   DeliveryFailureCause = 3
	CauseSCCongestion           DeliveryFailureCause = 4
	CauseInvalidSMEAddress      DeliveryFailureCause = 5
	CauseUserNotSCUser          DeliveryFailureCause = 6
)

var deliveryFailureCauseNames = tgpp.EnumNames{
	"MEMORY_CAPACITY_EXCEEDED", "EQUIPMENT_PROTOCOL_ERROR", "EQUIPMENT_NOT_SM_EQUIPPED", "UNKNOWN_SERVICE_CENTRE",
	"SC_CONGESTION", "INVALID_SME_ADDRESS", "USER_NOT_SC_USER",
}

func (c DeliveryFailureCause) String() string {
	return deliveryFailureCauseNames.Name("DeliveryFailureCause", uint32(c))
}

var (
	ErrMalformedAnswer = errors.New("sgd: malformed answer")
	ErrInvalidMessage  = errors.New("sgd: invalid message")
)

type Answer struct {
	SMRPUI []byte
}

type ResultError struct {
	tgpp.Result

	DeliveryFailureCause *DeliveryFailureCause
	DiagnosticInfo       []byte
	SMRPUI               []byte
	AbsentUserDiagnostic *tgpp.AbsentUserDiagnostic
}

func (e *ResultError) Error() string {
	return "sgd: request failed with " + e.String()
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %w", ErrInvalidMessage, fmt.Errorf(format, args...))
}

func checkSMRPUI(b []byte) error {
	if len(b) == 0 || len(b) > MaxSMRPUILength {
		return invalidf("SM-RP-UI of %d octets", len(b))
	}

	return nil
}

func smRPUIAVP(b []byte) diameter.AVP {
	return diameter.OctetString(AVPSMRPUI, diameter.AVPFlagMandatory, tgpp.VendorID, b)
}

func NewDeliveryFailureAnswer(req *diameter.Message, id diameter.Identity, cause DeliveryFailureCause, diagnostic []byte) (*diameter.Message, error) {
	if cause > CauseUserNotSCUser {
		return nil, invalidf("SM-Enumerated-Delivery-Failure-Cause %d", cause)
	}

	ans := tgpp.NewExperimentalAnswer(req, id, tgpp.ResultErrorSMDeliveryFailure)

	inner := []diameter.AVP{
		diameter.Unsigned32(AVPSMEnumeratedDeliveryFailureCause, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(cause)),
	}

	if diagnostic != nil {
		inner = append(inner, diameter.OctetString(AVPSMDiagnosticInfo, diameter.AVPFlagMandatory, tgpp.VendorID, diagnostic))
	}

	ans.AVPs = append(ans.AVPs, diameter.Grouped(AVPSMDeliveryFailureCause, diameter.AVPFlagMandatory, tgpp.VendorID, inner...))

	return ans, nil
}

func ParseMOForwardShortMessageAnswer(ans *diameter.Message) (Answer, error) {
	return parseAnswer(ans)
}

func ParseMTForwardShortMessageAnswer(ans *diameter.Message) (Answer, error) {
	return parseAnswer(ans)
}

func parseAnswer(ans *diameter.Message) (Answer, error) {
	result, err := tgpp.ParseFinalResult(ans)
	if err != nil {
		return Answer{}, fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	var smRPUI []byte

	if a, ok := ans.Find(AVPSMRPUI, tgpp.VendorID); ok {
		smRPUI = bytes.Clone(a.Data)
	}

	if result.Success() {
		return Answer{SMRPUI: smRPUI}, nil
	}

	e := &ResultError{Result: result, SMRPUI: smRPUI}

	if cause, ok := ans.Find(AVPSMDeliveryFailureCause, tgpp.VendorID); ok {
		if err := e.parseDeliveryFailureCause(cause); err != nil {
			return Answer{}, err
		}
	}

	if d, ok := ans.Find(tgpp.AVPAbsentUserDiagnosticSM, tgpp.VendorID); ok {
		v, err := d.Unsigned32()
		if err != nil {
			return Answer{}, fmt.Errorf("%w: Absent-User-Diagnostic-SM", ErrMalformedAnswer)
		}

		diagnostic := tgpp.AbsentUserDiagnostic(v)
		e.AbsentUserDiagnostic = &diagnostic
	}

	return Answer{}, e
}

func (e *ResultError) parseDeliveryFailureCause(cause diameter.AVP) error {
	inner, err := cause.Grouped()
	if err != nil {
		return fmt.Errorf("%w: SM-Delivery-Failure-Cause", ErrMalformedAnswer)
	}

	enum, ok := diameter.Find(inner, AVPSMEnumeratedDeliveryFailureCause, tgpp.VendorID)
	if !ok {
		return fmt.Errorf("%w: SM-Delivery-Failure-Cause without SM-Enumerated-Delivery-Failure-Cause", ErrMalformedAnswer)
	}

	v, err := enum.Unsigned32()
	if err != nil {
		return fmt.Errorf("%w: SM-Enumerated-Delivery-Failure-Cause", ErrMalformedAnswer)
	}

	c := DeliveryFailureCause(v)
	e.DeliveryFailureCause = &c

	if diag, ok := diameter.Find(inner, AVPSMDiagnosticInfo, tgpp.VendorID); ok {
		e.DiagnosticInfo = diag.Data
	}

	return nil
}
