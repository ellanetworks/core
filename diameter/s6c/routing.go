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
	MTI                  uint32
	GPRSIndicator        bool
	Priority             bool
	SingleAttempt        bool
	SMEA                 []byte
	DeliveryNotIntended  *uint32
	SMSFSupport          bool
}

type Routing struct {
	ServingNodes

	IMSI        string
	LMSI        []byte
	MWDStatus   uint32
	Absent      AbsentUserDiagnostics
	AlertMSISDN string
}

var srrRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: diameter.AVPUserName}:                                {},
	{Code: tgpp.AVPMSISDN, VendorID: tgpp.VendorID}:             {},
	{Code: tgpp.AVPSMSMICorrelationID, VendorID: tgpp.VendorID}: {},
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}:  {Multiple: true},
	{Code: tgpp.AVPSCAddress, VendorID: tgpp.VendorID}:          {Required: true},
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
		{r.GPRSIndicator, SRRFlagGPRSIndicator},
		{r.Priority, SRRFlagSMRPPRI},
		{r.SingleAttempt, SRRFlagSingleAttempt},
	} {
		if f.set {
			flags |= f.bit
		}
	}

	return flags
}

func NewSendRoutingInfoForSMRequest(env tgpp.Envelope, r RoutingRequest) (*diameter.Message, error) {
	if r.MSISDN == "" && r.IMSI == "" {
		return nil, invalid("routing request without an MSISDN or IMSI")
	}

	if r.MTI != SMRPMTIDeliver && r.MTI != SMRPMTIStatusReport {
		return nil, invalid("SM-RP-MTI %d", r.MTI)
	}

	if r.DeliveryNotIntended != nil && *r.DeliveryNotIntended > SMDeliveryNotIntendedMCCMNC {
		return nil, invalid("SM-Delivery-Not-Intended %d", *r.DeliveryNotIntended)
	}

	scAddress, err := tgpp.EncodeE164(r.ServiceCentreAddress)
	if err != nil {
		return nil, invalid("service centre address: %w", err)
	}

	avps := env.AVPs()

	if r.MSISDN != "" {
		msisdn, err := tgpp.EncodeE164(r.MSISDN)
		if err != nil {
			return nil, invalid("MSISDN: %w", err)
		}

		avps = append(avps, diameter.OctetString(tgpp.AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, msisdn))
	}

	if r.IMSI != "" {
		if !tgpp.ValidIMSI(r.IMSI) {
			return nil, invalid("IMSI %q", r.IMSI)
		}

		avps = append(avps, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, r.IMSI))
	}

	if r.SMSFSupport {
		avps = append(avps, smsfSupportFeature.AVP())
	}

	avps = append(avps,
		diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, scAddress),
		diameter.Unsigned32(AVPSMRPMTI, diameter.AVPFlagMandatory, tgpp.VendorID, r.MTI),
	)

	if len(r.SMEA) > 0 {
		avps = append(avps, diameter.OctetString(AVPSMRPSMEA, diameter.AVPFlagMandatory, tgpp.VendorID, r.SMEA))
	}

	if flags := r.flags(); flags != 0 {
		avps = append(avps, diameter.Unsigned32(AVPSRRFlags, diameter.AVPFlagMandatory, tgpp.VendorID, flags))
	}

	if r.DeliveryNotIntended != nil {
		avps = append(avps, diameter.Unsigned32(AVPSMDeliveryNotIntended, diameter.AVPFlagMandatory, tgpp.VendorID, *r.DeliveryNotIntended))
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
		return tgpp.MissingAVP(tgpp.AVPMSISDN, tgpp.VendorID)
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

	sc, _ := req.Find(tgpp.AVPSCAddress, tgpp.VendorID)
	if r.ServiceCentreAddress, err = tgpp.DecodeE164(sc.Data); err != nil {
		return RoutingRequest{}, tgpp.InvalidAVP(sc)
	}

	if a, ok := req.Find(AVPSMRPMTI, tgpp.VendorID); ok {
		if r.MTI, err = a.Unsigned32(); err != nil || r.MTI > SMRPMTIStatusReport {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}
	}

	if a, ok := req.Find(AVPSMRPSMEA, tgpp.VendorID); ok {
		r.SMEA = a.Data
	}

	if a, ok := req.Find(AVPSRRFlags, tgpp.VendorID); ok {
		flags, err := a.Unsigned32()
		if err != nil {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}

		r.GPRSIndicator = flags&SRRFlagGPRSIndicator != 0
		r.Priority = flags&SRRFlagSMRPPRI != 0
		r.SingleAttempt = flags&SRRFlagSingleAttempt != 0
	}

	if a, ok := req.Find(AVPSMDeliveryNotIntended, tgpp.VendorID); ok {
		v, err := a.Unsigned32()
		if err != nil || v > SMDeliveryNotIntendedMCCMNC {
			return RoutingRequest{}, tgpp.InvalidAVP(a)
		}

		r.DeliveryNotIntended = &v
	}

	r.SMSFSupport = smsfSupported(req)

	return r, nil
}

func NewSendRoutingInfoForSMAnswer(req *diameter.Message, id diameter.Identity, routing Routing, smsfSupport bool) (*diameter.Message, error) {
	if !tgpp.ValidIMSI(routing.IMSI) {
		return nil, invalid("routing answer IMSI %q", routing.IMSI)
	}

	if routing.Serving == nil && !routing.hasSMSF() {
		return nil, invalid("routing answer without a serving node")
	}

	if routing.Additional != nil && routing.Serving == nil {
		return nil, invalid("additional serving node without a serving node")
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
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPMWDStatus, diameter.AVPFlagMandatory, tgpp.VendorID, routing.MWDStatus))
	}

	ans.AVPs = append(ans.AVPs, routing.Absent.avps(smsfSupport)...)

	return ans, nil
}

func NewSendRoutingInfoForSMErrorAnswer(req *diameter.Message, id diameter.Identity, e ResultError, smsfSupport bool) (*diameter.Message, error) {
	if e.Success() {
		return nil, invalid("error answer with a success result")
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
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPMWDStatus, diameter.AVPFlagMandatory, tgpp.VendorID, e.MWDStatus))
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

		routing.MWDStatus = v
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
