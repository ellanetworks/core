// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type RoutingRequest struct {
	MSISDN               string
	IMSI                 string
	ServiceCentreAddress string
	MTI                  MTI
	GPRSIndicator        bool
	Priority             bool
	SingleAttempt        bool
	SMEA                 []byte
	DeliveryNotIntended  *DeliveryNotIntended
	SMSFSupport          bool
}

type Routing struct {
	ServingNodes

	IMSI        string
	LMSI        []byte
	MWDStatus   MWDStatus
	Absent      AbsentUserDiagnostics
	AlertMSISDN string
}

var srrRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: diameter.AVPUserName}:                                {},
	{Code: tgpp.AVPMSISDN, VendorID: tgpp.VendorID}:             {},
	{Code: tgpp.AVPSMSMICorrelationID, VendorID: tgpp.VendorID}: {},
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}:  {Multiple: true},
	{Code: tgpp.AVPSCAddress, VendorID: tgpp.VendorID}:          {},
	{Code: AVPSMRPMTI, VendorID: tgpp.VendorID}:                 {},
	{Code: AVPSMRPSMEA, VendorID: tgpp.VendorID}:                {},
	{Code: AVPSRRFlags, VendorID: tgpp.VendorID}:                {},
	{Code: AVPSMDeliveryNotIntended, VendorID: tgpp.VendorID}:   {},
})

func (r RoutingRequest) flags() uint32 {
	var flags uint32

	for _, f := range []struct {
		set bool
		bit uint32
	}{
		{r.GPRSIndicator, srrFlagGPRSIndicator},
		{r.Priority, srrFlagSMRPPRI},
		{r.SingleAttempt, srrFlagSingleAttempt},
	} {
		if f.set {
			flags |= f.bit
		}
	}

	return flags
}

func NewSendRoutingInfoForSMRequest(env tgpp.Envelope, r RoutingRequest) (*diameter.Message, error) {
	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

	if r.MSISDN == "" && r.IMSI == "" {
		return nil, invalidf("routing request without an MSISDN or IMSI")
	}

	if r.MTI != SMRPMTIDeliver && r.MTI != SMRPMTIStatusReport {
		return nil, invalidf("SM-RP-MTI %d", r.MTI)
	}

	if r.DeliveryNotIntended != nil && *r.DeliveryNotIntended > SMDeliveryNotIntendedMCCMNC {
		return nil, invalidf("SM-Delivery-Not-Intended %d", *r.DeliveryNotIntended)
	}

	avps := env.AVPs()

	if r.MSISDN != "" {
		msisdn, err := tgpp.EncodeE164(r.MSISDN)
		if err != nil {
			return nil, invalidf("MSISDN: %w", err)
		}

		avps = append(avps, diameter.OctetString(tgpp.AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, msisdn))
	}

	if r.IMSI != "" {
		if !tgpp.ValidIMSI(r.IMSI) {
			return nil, invalidf("IMSI %q", r.IMSI)
		}

		avps = append(avps, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, r.IMSI))
	}

	if r.SMSFSupport {
		avps = append(avps, smsfSupportFeature.AVP())
	}

	if r.ServiceCentreAddress != "" {
		scAddress, err := tgpp.EncodeE164(r.ServiceCentreAddress)
		if err != nil {
			return nil, invalidf("service centre address: %w", err)
		}

		avps = append(avps, diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, scAddress))
	}

	avps = append(avps, diameter.Unsigned32(AVPSMRPMTI, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(r.MTI)))

	if len(r.SMEA) > 0 {
		avps = append(avps, diameter.OctetString(AVPSMRPSMEA, diameter.AVPFlagMandatory, tgpp.VendorID, r.SMEA))
	}

	if flags := r.flags(); flags != 0 {
		avps = append(avps, diameter.Unsigned32(AVPSRRFlags, diameter.AVPFlagMandatory, tgpp.VendorID, flags))
	}

	if r.DeliveryNotIntended != nil {
		avps = append(avps, diameter.Unsigned32(AVPSMDeliveryNotIntended, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(*r.DeliveryNotIntended)))
	}

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   CommandSendRoutingInfoForSM,
		ApplicationID: ApplicationID,
		AVPs:          avps,
	}, nil
}

func CheckSendRoutingInfoForSM(req *diameter.Message) error {
	if err := srrRules.Check(req); err != nil {
		return err
	}

	_, hasMSISDN := req.Find(tgpp.AVPMSISDN, tgpp.VendorID)
	_, hasUserName := req.Find(diameter.AVPUserName, 0)

	if !hasMSISDN && !hasUserName {
		return tgpp.MissingAVP(tgpp.AVPMSISDN, tgpp.VendorID, 0)
	}

	return nil
}

func ParseSendRoutingInfoForSMRequest(req *diameter.Message) (RoutingRequest, error) {
	if err := CheckSendRoutingInfoForSM(req); err != nil {
		return RoutingRequest{}, err
	}

	var (
		r   RoutingRequest
		err error
	)

	if a, ok := req.Find(tgpp.AVPMSISDN, tgpp.VendorID); ok {
		if r.MSISDN, err = tgpp.DecodeE164(a.Data); err != nil {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}
	}

	if a, ok := req.Find(diameter.AVPUserName, 0); ok {
		if r.IMSI = a.UTF8String(); !tgpp.ValidIMSI(r.IMSI) {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}
	}

	if sc, ok := req.Find(tgpp.AVPSCAddress, tgpp.VendorID); ok {
		if r.ServiceCentreAddress, err = tgpp.DecodeE164(sc.Data); err != nil {
			return RoutingRequest{}, tgpp.InvalidAVP(sc)
		}
	}

	if a, ok := req.Find(AVPSMRPMTI, tgpp.VendorID); ok {
		mti, err := tgpp.Unsigned32(a)
		if err != nil {
			return RoutingRequest{}, err
		}

		if MTI(mti) > SMRPMTIStatusReport {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}

		r.MTI = MTI(mti)
	}

	if a, ok := req.Find(AVPSMRPSMEA, tgpp.VendorID); ok {
		r.SMEA = a.Data
	}

	if a, ok := req.Find(AVPSRRFlags, tgpp.VendorID); ok {
		flags, err := tgpp.Unsigned32(a)
		if err != nil {
			return RoutingRequest{}, err
		}

		r.GPRSIndicator = flags&srrFlagGPRSIndicator != 0
		r.Priority = flags&srrFlagSMRPPRI != 0
		r.SingleAttempt = flags&srrFlagSingleAttempt != 0
	}

	if a, ok := req.Find(AVPSMDeliveryNotIntended, tgpp.VendorID); ok {
		v, err := tgpp.Unsigned32(a)
		if err != nil {
			return RoutingRequest{}, err
		}

		if DeliveryNotIntended(v) > SMDeliveryNotIntendedMCCMNC {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}

		notIntended := DeliveryNotIntended(v)
		r.DeliveryNotIntended = &notIntended
	}

	r.SMSFSupport = smsfSupported(req)

	return r, nil
}

func NewSendRoutingInfoForSMAnswer(req *diameter.Message, id diameter.Identity, routing Routing, smsfSupport bool) (*diameter.Message, error) {
	if !tgpp.ValidIMSI(routing.IMSI) {
		return nil, invalidf("routing answer IMSI %q", routing.IMSI)
	}

	if _, notIntended := req.Find(AVPSMDeliveryNotIntended, tgpp.VendorID); routing.Serving == nil && !routing.hasSMSF() && !notIntended {
		return nil, invalidf("routing answer without a serving node")
	}

	if routing.Additional != nil && routing.Serving == nil {
		return nil, invalidf("additional serving node without a serving node")
	}

	nodes, err := routing.avps(smsfSupport)
	if err != nil {
		return nil, err
	}

	alert, err := alertMSISDNAVP(routing.AlertMSISDN)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, tgpp.Result{Code: diameter.ResultSuccess})
	ans.AVPs = append(ans.AVPs, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, routing.IMSI))
	ans.AVPs = append(ans.AVPs, nodes...)

	if routing.LMSI != nil {
		ans.AVPs = append(ans.AVPs, diameter.OctetString(AVPLMSI, diameter.AVPFlagMandatory, tgpp.VendorID, routing.LMSI))
	}

	ans.AVPs = append(ans.AVPs, alert...)

	if routing.MWDStatus != 0 {
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPMWDStatus, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(routing.MWDStatus)))
	}

	ans.AVPs = append(ans.AVPs, routing.Absent.avps(smsfSupport)...)

	return ans, nil
}

func NewSendRoutingInfoForSMErrorAnswer(req *diameter.Message, id diameter.Identity, e ResultError, smsfSupport bool) (*diameter.Message, error) {
	if !e.Failure() {
		return nil, invalidf("error answer with the non-error %s", e.Result)
	}

	ans := NewAnswer(req, id, e.Result)

	if !e.Experimental {
		return ans, nil
	}

	alert, err := alertMSISDNAVP(e.AlertMSISDN)
	if err != nil {
		return nil, err
	}

	ans.AVPs = append(ans.AVPs, alert...)

	if e.MWDStatus != 0 {
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPMWDStatus, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(e.MWDStatus)))
	}

	ans.AVPs = append(ans.AVPs, e.Absent.avps(smsfSupport)...)

	return ans, nil
}

func ParseSendRoutingInfoForSMAnswer(ans *diameter.Message) (Routing, error) {
	if err := parseResult(ans); err != nil {
		return Routing{}, err
	}

	userName, ok := ans.Find(diameter.AVPUserName, 0)
	if !ok || userName.UTF8String() == "" {
		return Routing{}, fmt.Errorf("%w: no User-Name", ErrMalformedAnswer)
	}

	routing := Routing{IMSI: userName.UTF8String()}

	if lmsi, ok := ans.Find(AVPLMSI, tgpp.VendorID); ok {
		routing.LMSI = lmsi.Data
	}

	if mwd, ok := ans.Find(AVPMWDStatus, tgpp.VendorID); ok {
		v, err := mwd.Unsigned32()
		if err != nil {
			return Routing{}, fmt.Errorf("%w: MWD-Status", ErrMalformedAnswer)
		}

		routing.MWDStatus = MWDStatus(v)
	}

	routing.Absent = absentUserDiagnostics(ans)

	var err error

	if routing.AlertMSISDN, err = alertMSISDN(ans); err != nil {
		return Routing{}, err
	}

	if routing.ServingNodes, err = parseServingNodes(ans); err != nil {
		return Routing{}, fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	if routing.empty() {
		return Routing{}, fmt.Errorf("%w: no serving node", ErrMalformedAnswer)
	}

	return routing, nil
}

func requestServingNodes(req *diameter.Message) (ServingNodes, error) {
	nodes, err := parseServingNodes(req)

	var pe *avpParseError
	if errors.As(err, &pe) {
		return ServingNodes{}, tgpp.InvalidAVP(pe.avp)
	}

	return nodes, err
}
