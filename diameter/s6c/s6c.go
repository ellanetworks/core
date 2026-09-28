// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const ApplicationID uint32 = 16777312

const (
	CommandSendRoutingInfoForSM   uint32 = 8388647
	CommandAlertServiceCentre     uint32 = 8388648
	CommandReportSMDeliveryStatus uint32 = 8388649
)

const (
	AVPLMSI                              uint32 = 2400
	AVPServingNode                       uint32 = 2401
	AVPMMEName                           uint32 = 2402
	AVPMSCNumber                         uint32 = 2403
	AVPAdditionalServingNode             uint32 = 2406
	AVPMMERealm                          uint32 = 2408
	AVPSGSNName                          uint32 = 2409
	AVPSGSNRealm                         uint32 = 2410
	AVPIPSMGWNumber                      uint32 = 3100
	AVPIPSMGWName                        uint32 = 3101
	AVPIPSMGWRealm                       uint32 = 3112
	AVPSMRPMTI                           uint32 = 3308
	AVPSMRPSMEA                          uint32 = 3309
	AVPSRRFlags                          uint32 = 3310
	AVPSMDeliveryNotIntended             uint32 = 3311
	AVPMWDStatus                         uint32 = 3312
	AVPMMEAbsentUserDiagnosticSM         uint32 = 3313
	AVPMSCAbsentUserDiagnosticSM         uint32 = 3314
	AVPSGSNAbsentUserDiagnosticSM        uint32 = 3315
	AVPMMESMDeliveryOutcome              uint32 = 3317
	AVPMSCSMDeliveryOutcome              uint32 = 3318
	AVPSGSNSMDeliveryOutcome             uint32 = 3319
	AVPIPSMGWSMDeliveryOutcome           uint32 = 3320
	AVPSMDeliveryCause                   uint32 = 3321
	AVPRDRFlags                          uint32 = 3323
	AVPMaximumUEAvailabilityTime         uint32 = 3329
	AVPSMSGMSCAlertEvent                 uint32 = 3333
	AVPSMSF3GPPAbsentUserDiagnosticSM    uint32 = 3334
	AVPSMSFNon3GPPAbsentUserDiagnosticSM uint32 = 3335
	AVPSMSF3GPPSMDeliveryOutcome         uint32 = 3336
	AVPSMSFNon3GPPSMDeliveryOutcome      uint32 = 3337
	AVPSMSF3GPPNumber                    uint32 = 3338
	AVPSMSFNon3GPPNumber                 uint32 = 3339
	AVPSMSF3GPPName                      uint32 = 3340
	AVPSMSFNon3GPPName                   uint32 = 3341
	AVPSMSF3GPPRealm                     uint32 = 3342
	AVPSMSFNon3GPPRealm                  uint32 = 3343
	AVPSMSF3GPPAddress                   uint32 = 3344
	AVPSMSFNon3GPPAddress                uint32 = 3345
)

const (
	SMRPMTIDeliver      uint32 = 0
	SMRPMTIStatusReport uint32 = 1
)

const (
	SRRFlagGPRSIndicator uint32 = 1 << 0
	SRRFlagSMRPPRI       uint32 = 1 << 1
	SRRFlagSingleAttempt uint32 = 1 << 2
)

const (
	SMDeliveryNotIntendedIMSI   uint32 = 0
	SMDeliveryNotIntendedMCCMNC uint32 = 1
)

const RDRFlagSingleAttempt uint32 = 1 << 0

const (
	AlertEventUEAvailableForMTSMS uint32 = 1 << 0
	AlertEventUEUnderNewNode      uint32 = 1 << 1
)

const (
	FeatureListID      uint32 = 1
	FeatureSMSFSupport uint32 = 1 << 0
)

const (
	MWDStatusSCAddressNotIncluded uint32 = 1 << 0
	MWDStatusMNRF                 uint32 = 1 << 1
	MWDStatusMCEF                 uint32 = 1 << 2
	MWDStatusMNRG                 uint32 = 1 << 3
	MWDStatusMNR5G                uint32 = 1 << 4
	MWDStatusMNR5GN3G             uint32 = 1 << 5
)

const (
	DeliveryCauseMemoryCapacityExceeded uint32 = 0
	DeliveryCauseAbsentUser             uint32 = 1
	DeliveryCauseSuccessfulTransfer     uint32 = 2
)

var (
	ErrMalformedAnswer = errors.New("s6c: malformed answer")
	ErrInvalidMessage  = errors.New("s6c: invalid message")
)

var smsfSupportFeature = tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: FeatureListID, FeatureList: FeatureSMSFSupport}

type AbsentUserDiagnostics struct {
	MME         *uint32
	MSC         *uint32
	SGSN        *uint32
	SMSF3GPP    *uint32
	SMSFNon3GPP *uint32
}

type ResultError struct {
	tgpp.Result

	MWDStatus   uint32
	Absent      AbsentUserDiagnostics
	AlertMSISDN string
}

func (e *ResultError) Error() string {
	return "s6c: request failed with " + e.String()
}

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result) *diameter.Message {
	return withFeatures(tgpp.NewResultAnswer(req, id, r))
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error) *diameter.Message {
	return withFeatures(tgpp.NewErrorAnswer(req, id, err))
}

func withFeatures(ans *diameter.Message) *diameter.Message {
	ans.AVPs = append(ans.AVPs, smsfSupportFeature.AVP())

	return ans
}

func smsfSupported(m *diameter.Message) bool {
	return tgpp.FeatureList(m.AVPs, tgpp.VendorID, FeatureListID)&FeatureSMSFSupport != 0
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidMessage}, args...)...)
}

func parseResult(ans *diameter.Message) error {
	result, err := tgpp.ParseResult(ans)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	if result.Success() {
		return nil
	}

	e := &ResultError{Result: result}

	if !result.Experimental {
		return e
	}

	if e.AlertMSISDN, err = alertMSISDN(ans); err != nil {
		return err
	}

	if mwd, ok := ans.Find(AVPMWDStatus, tgpp.VendorID); ok {
		if e.MWDStatus, err = mwd.Unsigned32(); err != nil {
			return fmt.Errorf("%w: MWD-Status", ErrMalformedAnswer)
		}
	}

	e.Absent = absentUserDiagnostics(ans)

	return e
}

func alertMSISDN(m *diameter.Message) (string, error) {
	a, ok := m.Find(tgpp.AVPUserIdentifier, tgpp.VendorID)
	if !ok {
		return "", nil
	}

	u, err := tgpp.ParseUserIdentifier(a)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	return u.MSISDN, nil
}

func alertMSISDNAVP(msisdn string) ([]diameter.AVP, error) {
	if msisdn == "" {
		return nil, nil
	}

	ui, err := tgpp.NewUserIdentifier(tgpp.UserIdentifier{MSISDN: msisdn})
	if err != nil {
		return nil, invalid("alert MSISDN: %w", err)
	}

	return []diameter.AVP{ui}, nil
}

type absentField struct {
	code  uint32
	flags uint8
	value **uint32
	smsf  bool
}

func (a *AbsentUserDiagnostics) fields() []absentField {
	return []absentField{
		{AVPMMEAbsentUserDiagnosticSM, diameter.AVPFlagMandatory, &a.MME, false},
		{AVPMSCAbsentUserDiagnosticSM, diameter.AVPFlagMandatory, &a.MSC, false},
		{AVPSGSNAbsentUserDiagnosticSM, diameter.AVPFlagMandatory, &a.SGSN, false},
		{AVPSMSF3GPPAbsentUserDiagnosticSM, 0, &a.SMSF3GPP, true},
		{AVPSMSFNon3GPPAbsentUserDiagnosticSM, 0, &a.SMSFNon3GPP, true},
	}
}

func absentUserDiagnostics(ans *diameter.Message) AbsentUserDiagnostics {
	var d AbsentUserDiagnostics

	for _, f := range d.fields() {
		a, ok := ans.Find(f.code, tgpp.VendorID)
		if !ok {
			continue
		}

		if v, err := a.Unsigned32(); err == nil {
			*f.value = &v
		}
	}

	return d
}

func (a AbsentUserDiagnostics) avps(smsfSupport bool) []diameter.AVP {
	var avps []diameter.AVP

	for _, f := range a.fields() {
		if *f.value != nil && (smsfSupport || !f.smsf) {
			avps = append(avps, diameter.Unsigned32(f.code, f.flags, tgpp.VendorID, **f.value))
		}
	}

	return avps
}
