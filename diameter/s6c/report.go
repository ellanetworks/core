// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type DeliveryOutcome struct {
	Cause            DeliveryCause
	AbsentDiagnostic *tgpp.AbsentUserDiagnostic
}

type DeliveryReport struct {
	MSISDN               string
	IMSI                 string
	ServiceCentreAddress string
	SingleAttempt        bool
	MME                  *DeliveryOutcome
	MSC                  *DeliveryOutcome
	SGSN                 *DeliveryOutcome
	IPSMGW               *DeliveryOutcome
	SMSF3GPP             *DeliveryOutcome
	SMSFNon3GPP          *DeliveryOutcome
	Failed               ServingNodes
	SMSFSupport          bool
}

type ReportResult struct {
	ServingNodes

	AlertMSISDN string
}

var rdrRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}:  {Multiple: true},
	{Code: tgpp.AVPUserIdentifier, VendorID: tgpp.VendorID}:     {Required: true},
	{Code: tgpp.AVPSMSMICorrelationID, VendorID: tgpp.VendorID}: {},
	{Code: tgpp.AVPSCAddress, VendorID: tgpp.VendorID}:          {Required: true},
	{Code: tgpp.AVPSMDeliveryOutcome, VendorID: tgpp.VendorID}:  {Required: true},
	{Code: AVPRDRFlags, VendorID: tgpp.VendorID}:                {},
	{Code: AVPServingNode, VendorID: tgpp.VendorID}:             {},
	{Code: AVPAdditionalServingNode, VendorID: tgpp.VendorID}:   {},
	{Code: AVPSMSF3GPPAddress, VendorID: tgpp.VendorID}:         {},
	{Code: AVPSMSFNon3GPPAddress, VendorID: tgpp.VendorID}:      {},
})

type outcomeField struct {
	code    uint32
	flags   uint8
	outcome **DeliveryOutcome
}

func (rep *DeliveryReport) outcomes() []outcomeField {
	return []outcomeField{
		{AVPMMESMDeliveryOutcome, diameter.AVPFlagMandatory, &rep.MME},
		{AVPMSCSMDeliveryOutcome, diameter.AVPFlagMandatory, &rep.MSC},
		{AVPSGSNSMDeliveryOutcome, diameter.AVPFlagMandatory, &rep.SGSN},
		{AVPIPSMGWSMDeliveryOutcome, diameter.AVPFlagMandatory, &rep.IPSMGW},
		{AVPSMSF3GPPSMDeliveryOutcome, 0, &rep.SMSF3GPP},
		{AVPSMSFNon3GPPSMDeliveryOutcome, 0, &rep.SMSFNon3GPP},
	}
}

func (rep *DeliveryReport) validateOutcomes() error {
	found := false

	for _, f := range rep.outcomes() {
		if *f.outcome == nil {
			continue
		}

		if (*f.outcome).Cause > DeliveryCauseSuccessfulTransfer {
			return invalidf("SM-Delivery-Cause %d", (*f.outcome).Cause)
		}

		found = true
	}

	switch {
	case !found:
		return invalidf("delivery report without an outcome")
	case rep.MME != nil && rep.MSC != nil:
		return invalidf("delivery report with both an MME and an MSC outcome")
	}

	return nil
}

func NewReportSMDeliveryStatusRequest(env tgpp.Envelope, rep DeliveryReport) (*diameter.Message, error) {
	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

	ui, err := tgpp.NewUserIdentifier(tgpp.UserIdentifier{MSISDN: rep.MSISDN, IMSI: rep.IMSI})
	if err != nil {
		return nil, invalidf("%w", err)
	}

	scAddress, err := tgpp.EncodeE164(rep.ServiceCentreAddress)
	if err != nil {
		return nil, invalidf("service centre address: %w", err)
	}

	if err := rep.validateOutcomes(); err != nil {
		return nil, err
	}

	if rep.Failed.Additional != nil && rep.Failed.Serving == nil {
		return nil, invalidf("additional failed serving node without a serving node")
	}

	failed, err := rep.Failed.avps(rep.SMSFSupport)
	if err != nil {
		return nil, err
	}

	var outcomes []diameter.AVP

	for _, f := range rep.outcomes() {
		if *f.outcome != nil {
			outcomes = append(outcomes, deliveryOutcome(f.code, f.flags, **f.outcome))
		}
	}

	avps := env.AVPs()

	if rep.SMSFSupport {
		avps = append(avps, smsfSupportFeature.AVP())
	}

	avps = append(avps,
		ui,
		diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, scAddress),
		diameter.Grouped(tgpp.AVPSMDeliveryOutcome, diameter.AVPFlagMandatory, tgpp.VendorID, outcomes...),
	)

	if rep.SingleAttempt {
		avps = append(avps, diameter.Unsigned32(AVPRDRFlags, 0, tgpp.VendorID, RDRFlagSingleAttempt))
	}

	avps = append(avps, failed...)

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   CommandReportSMDeliveryStatus,
		ApplicationID: ApplicationID,
		AVPs:          avps,
	}, nil
}

func deliveryOutcome(code uint32, flags uint8, o DeliveryOutcome) diameter.AVP {
	inner := []diameter.AVP{
		diameter.Unsigned32(AVPSMDeliveryCause, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(o.Cause)),
	}

	if o.AbsentDiagnostic != nil {
		inner = append(inner, diameter.Unsigned32(tgpp.AVPAbsentUserDiagnosticSM, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(*o.AbsentDiagnostic)))
	}

	return diameter.Grouped(code, flags, tgpp.VendorID, inner...)
}

func CheckReportSMDeliveryStatus(req *diameter.Message) error {
	return rdrRules.Check(req)
}

func ParseReportSMDeliveryStatusRequest(req *diameter.Message) (DeliveryReport, error) {
	if err := CheckReportSMDeliveryStatus(req); err != nil {
		return DeliveryReport{}, err
	}

	var rep DeliveryReport

	ui, _ := req.Find(tgpp.AVPUserIdentifier, tgpp.VendorID)

	user, err := tgpp.ParseUserIdentifier(ui)
	if err != nil {
		return DeliveryReport{}, tgpp.InvalidAVP(ui)
	}

	rep.MSISDN, rep.IMSI = user.MSISDN, user.IMSI

	sc, _ := req.Find(tgpp.AVPSCAddress, tgpp.VendorID)
	if rep.ServiceCentreAddress, err = tgpp.DecodeE164(sc.Data); err != nil {
		return DeliveryReport{}, tgpp.InvalidAVP(sc)
	}

	outcome, _ := req.Find(tgpp.AVPSMDeliveryOutcome, tgpp.VendorID)
	if !parseDeliveryOutcomes(outcome, &rep) || rep.validateOutcomes() != nil {
		return DeliveryReport{}, tgpp.InvalidAVP(outcome)
	}

	if a, ok := req.Find(AVPRDRFlags, tgpp.VendorID); ok {
		flags, err := tgpp.Unsigned32(a)
		if err != nil {
			return DeliveryReport{}, err
		}

		rep.SingleAttempt = flags&RDRFlagSingleAttempt != 0
	}

	if rep.Failed, err = requestServingNodes(req); err != nil {
		return DeliveryReport{}, err
	}

	rep.SMSFSupport = smsfSupported(req)

	return rep, nil
}

func parseDeliveryOutcomes(a diameter.AVP, rep *DeliveryReport) bool {
	inner, err := a.Grouped()
	if err != nil {
		return false
	}

	for _, f := range rep.outcomes() {
		o, ok := diameter.Find(inner, f.code, tgpp.VendorID)
		if !ok {
			continue
		}

		outcome, ok := parseDeliveryOutcome(o)
		if !ok {
			return false
		}

		*f.outcome = outcome
	}

	return true
}

func parseDeliveryOutcome(a diameter.AVP) (*DeliveryOutcome, bool) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, false
	}

	cause, ok := diameter.Find(inner, AVPSMDeliveryCause, tgpp.VendorID)
	if !ok {
		return nil, false
	}

	o := &DeliveryOutcome{}

	c, err := cause.Unsigned32()
	if err != nil {
		return nil, false
	}

	o.Cause = DeliveryCause(c)

	if d, ok := diameter.Find(inner, tgpp.AVPAbsentUserDiagnosticSM, tgpp.VendorID); ok {
		v, err := d.Unsigned32()
		if err != nil {
			return nil, false
		}

		diagnostic := tgpp.AbsentUserDiagnostic(v)
		o.AbsentDiagnostic = &diagnostic
	}

	return o, true
}

func NewReportSMDeliveryStatusAnswer(req *diameter.Message, id diameter.Identity, res ReportResult, smsfSupport bool) (*diameter.Message, error) {
	if res.Additional != nil && res.Serving == nil {
		return nil, invalidf("additional serving node without a serving node")
	}

	nodes, err := res.avps(smsfSupport)
	if err != nil {
		return nil, err
	}

	alert, err := alertMSISDNAVP(res.AlertMSISDN)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, tgpp.Result{Code: diameter.ResultSuccess})
	ans.AVPs = append(ans.AVPs, alert...)
	ans.AVPs = append(ans.AVPs, nodes...)

	return ans, nil
}

func ParseReportSMDeliveryStatusAnswer(ans *diameter.Message) (ReportResult, error) {
	if err := parseResult(ans); err != nil {
		return ReportResult{}, err
	}

	var (
		result ReportResult
		err    error
	)

	if result.AlertMSISDN, err = alertMSISDN(ans); err != nil {
		return ReportResult{}, err
	}

	if result.ServingNodes, err = parseServingNodes(ans); err != nil {
		return ReportResult{}, fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	return result, nil
}
