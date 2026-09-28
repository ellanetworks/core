// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
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

const TFRFlagMoreMessagesToSend uint32 = 1 << 0

const (
	CauseMemoryCapacityExceeded uint32 = 0
	CauseEquipmentProtocolError uint32 = 1
	CauseEquipmentNotSMEquipped uint32 = 2
	CauseUnknownServiceCentre   uint32 = 3
	CauseSCCongestion           uint32 = 4
	CauseInvalidSMEAddress      uint32 = 5
	CauseUserNotSCUser          uint32 = 6
)

var (
	ErrMalformedAnswer = errors.New("sgd: malformed answer")
	ErrInvalidMessage  = errors.New("sgd: invalid message")
)

type Answer struct {
	SMRPUI []byte
}

type ResultError struct {
	tgpp.Result

	DeliveryFailureCause *uint32
	DiagnosticInfo       []byte
	SMRPUI               []byte
	AbsentUserDiagnostic *uint32
}

func (e *ResultError) Error() string {
	return "sgd: request failed with " + e.String()
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidMessage}, args...)...)
}

func checkSMRPUI(b []byte) error {
	if len(b) == 0 || len(b) > MaxSMRPUILength {
		return invalid("SM-RP-UI of %d octets", len(b))
	}

	return nil
}

func smRPUIAVP(b []byte) diameter.AVP {
	return diameter.OctetString(AVPSMRPUI, diameter.AVPFlagMandatory, tgpp.VendorID, b)
}

func NewDeliveryFailureAnswer(req *diameter.Message, id diameter.Identity, cause uint32, diagnostic []byte) (*diameter.Message, error) {
	if cause > CauseUserNotSCUser {
		return nil, invalid("SM-Enumerated-Delivery-Failure-Cause %d", cause)
	}

	ans := tgpp.NewExperimentalAnswer(req, id, tgpp.ResultErrorSMDeliveryFailure)

	inner := []diameter.AVP{
		diameter.Unsigned32(AVPSMEnumeratedDeliveryFailureCause, diameter.AVPFlagMandatory, tgpp.VendorID, cause),
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
	result, err := tgpp.ParseResult(ans)
	if err != nil {
		return Answer{}, fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	var smRPUI []byte

	if a, ok := ans.Find(AVPSMRPUI, tgpp.VendorID); ok {
		smRPUI = a.Data
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

		e.AbsentUserDiagnostic = &v
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

	e.DeliveryFailureCause = &v

	if diag, ok := diameter.Find(inner, AVPSMDiagnosticInfo, tgpp.VendorID); ok {
		e.DiagnosticInfo = diag.Data
	}

	return nil
}
