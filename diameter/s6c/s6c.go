// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"errors"
	"fmt"
	"strings"

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

type MTI uint32

const (
	SMRPMTIDeliver      MTI = 0
	SMRPMTIStatusReport MTI = 1
)

var mtiNames = map[MTI]string{
	SMRPMTIDeliver:      "SM_DELIVER",
	SMRPMTIStatusReport: "SM_STATUS_REPORT",
}

func (m MTI) String() string {
	if name, ok := mtiNames[m]; ok {
		return name
	}

	return fmt.Sprintf("MTI(%d)", uint32(m))
}

const (
	srrFlagGPRSIndicator uint32 = 1 << 0
	srrFlagSMRPPRI       uint32 = 1 << 1
	srrFlagSingleAttempt uint32 = 1 << 2
)

type DeliveryNotIntended uint32

const (
	SMDeliveryNotIntendedIMSI   DeliveryNotIntended = 0
	SMDeliveryNotIntendedMCCMNC DeliveryNotIntended = 1
)

var deliveryNotIntendedNames = map[DeliveryNotIntended]string{
	SMDeliveryNotIntendedIMSI:   "ONLY_IMSI_REQUESTED",
	SMDeliveryNotIntendedMCCMNC: "ONLY_MCC_MNC_REQUESTED",
}

func (d DeliveryNotIntended) String() string {
	if name, ok := deliveryNotIntendedNames[d]; ok {
		return name
	}

	return fmt.Sprintf("DeliveryNotIntended(%d)", uint32(d))
}

const rdrFlagSingleAttempt uint32 = 1 << 0

type AlertEvent uint32

const (
	AlertEventUEAvailableForMTSMS AlertEvent = 1 << 0
	AlertEventUEUnderNewNode      AlertEvent = 1 << 1
)

var alertEventNames = []string{
	"UE_AVAILABLE_FOR_MT_SMS",
	"UE_UNDER_NEW_SERVING_NODE",
}

func (e AlertEvent) String() string {
	return bitNames(uint32(e), alertEventNames)
}

const (
	featureListID      uint32 = 1
	featureSMSFSupport uint32 = 1 << 0
)

type MWDStatus uint32

const (
	MWDStatusSCAddressNotIncluded MWDStatus = 1 << 0
	MWDStatusMNRF                 MWDStatus = 1 << 1
	MWDStatusMCEF                 MWDStatus = 1 << 2
	MWDStatusMNRG                 MWDStatus = 1 << 3
	MWDStatusMNR5G                MWDStatus = 1 << 4
	MWDStatusMNR5GN3G             MWDStatus = 1 << 5
)

var mwdStatusNames = []string{
	"SC_ADDRESS_NOT_INCLUDED",
	"MNRF_SET",
	"MCEF_SET",
	"MNRG_SET",
	"MNR5G_SET",
	"MNR5GN3G_SET",
}

func (s MWDStatus) String() string {
	return bitNames(uint32(s), mwdStatusNames)
}

type DeliveryCause uint32

const (
	DeliveryCauseMemoryCapacityExceeded DeliveryCause = 0
	DeliveryCauseAbsentUser             DeliveryCause = 1
	DeliveryCauseSuccessfulTransfer     DeliveryCause = 2
)

var deliveryCauseNames = map[DeliveryCause]string{
	DeliveryCauseMemoryCapacityExceeded: "UE_MEMORY_CAPACITY_EXCEEDED",
	DeliveryCauseAbsentUser:             "ABSENT_USER",
	DeliveryCauseSuccessfulTransfer:     "SUCCESSFUL_TRANSFER",
}

func (c DeliveryCause) String() string {
	if name, ok := deliveryCauseNames[c]; ok {
		return name
	}

	return fmt.Sprintf("DeliveryCause(%d)", uint32(c))
}

func bitNames(v uint32, names []string) string {
	if v == 0 {
		return "0"
	}

	var parts []string

	for i, name := range names {
		if bit := uint32(1) << i; v&bit != 0 {
			parts = append(parts, name)
			v &^= bit
		}
	}

	if v != 0 {
		parts = append(parts, fmt.Sprintf("%#x", v))
	}

	return strings.Join(parts, "|")
}

var (
	ErrMalformedAnswer = errors.New("s6c: malformed answer")
	ErrInvalidMessage  = errors.New("s6c: invalid message")
)

var smsfSupportFeature = tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: featureListID, FeatureList: featureSMSFSupport}

type AbsentUserDiagnostics struct {
	MME         *tgpp.AbsentUserDiagnostic
	MSC         *tgpp.AbsentUserDiagnostic
	SGSN        *tgpp.AbsentUserDiagnostic
	SMSF3GPP    *tgpp.AbsentUserDiagnostic
	SMSFNon3GPP *tgpp.AbsentUserDiagnostic
}

type ResultError struct {
	tgpp.Result

	MWDStatus   MWDStatus
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
	return tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureListID)&featureSMSFSupport != 0
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
		v, err := mwd.Unsigned32()
		if err != nil {
			return fmt.Errorf("%w: MWD-Status", ErrMalformedAnswer)
		}

		e.MWDStatus = MWDStatus(v)
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
	value **tgpp.AbsentUserDiagnostic
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
			diagnostic := tgpp.AbsentUserDiagnostic(v)
			*f.value = &diagnostic
		}
	}

	return d
}

func (a AbsentUserDiagnostics) avps(smsfSupport bool) []diameter.AVP {
	var avps []diameter.AVP

	for _, f := range a.fields() {
		if *f.value != nil && (smsfSupport || !f.smsf) {
			avps = append(avps, diameter.Unsigned32(f.code, f.flags, tgpp.VendorID, uint32(**f.value)))
		}
	}

	return avps
}
