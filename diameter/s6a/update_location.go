// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type UpdateLocationRequest struct {
	IMSI                      string
	Features                  Features
	Terminal                  *TerminalInformation
	RATType                   tgpp.RATType
	Flags                     ULRFlags
	UESRVCCCapability         *UESRVCCCapability
	VisitedPLMN               tgpp.PLMN
	SGSNNumber                string
	HomogeneousIMSVoiceOverPS *IMSVoiceOverPS
	EquivalentPLMNs           []tgpp.PLMN
	MMENumberForMTSMS         string
	SMSRegisterRequest        *SMSRegisterRequest
	SGsMMEIdentity            string
	CoupledNodeDiameterID     string
}

type UpdateLocation struct {
	HSS          diameter.Identity
	Features     Features
	Flags        ULAFlags
	Subscription *SubscriptionData
	ResetIDs     []ResetID
}

var (
	ulrRules = requestRules(false, userNameRule.With(diameter.Rules{
		vendorKey(AVPTerminalInformation):                       {},
		vendorKey(tgpp.AVPRATType):                              {Required: true, MinLength: 4},
		vendorKey(AVPULRFlags):                                  {Required: true, MinLength: 4},
		vendorKey(AVPUESRVCCCapability):                         {},
		vendorKey(AVPVisitedPLMNID):                             {Required: true, MinLength: 3},
		vendorKey(tgpp.AVPSGSNNumber):                           {},
		vendorKey(AVPHomogeneousSupportOfIMSVoiceOverPSSession): {},
		vendorKey(AVPGMLCAddress):                               {},
		vendorKey(AVPActiveAPN):                                 {Multiple: true},
		vendorKey(AVPEquivalentPLMNList):                        {},
		vendorKey(tgpp.AVPMMENumberForMTSMS):                    {},
		vendorKey(AVPSMSRegisterRequest):                        {},
		vendorKey(AVPSGsMMEIdentity):                            {},
		vendorKey(AVPCoupledNodeDiameterID):                     {},
		vendorKey(AVPAdjacentPLMNs):                             {},
		vendorKey(AVPSupportedServices):                         {},
		vendorKey(AVPSFULRTimestamp):                            {},
		vendorKey(AVPSFProvisionalIndication):                   {},
	}))
	equivalentPLMNListRules = diameter.Rules{vendorKey(AVPVisitedPLMNID): {Multiple: true}}
)

func NewUpdateLocationRequest(env tgpp.Envelope, r UpdateLocationRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	if !r.RATType.ThreeGPP() {
		return nil, invalidf("RAT-Type %s is not a 3GPP radio access technology", r.RATType)
	}

	if !r.Flags.Known() {
		return nil, invalidf("ULR-Flags with undefined bits %#x", uint32(r.Flags&^ulrFlagsDefined))
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)

	if r.Terminal != nil {
		terminal, err := r.Terminal.avp()
		if err != nil {
			return nil, err
		}

		avps = append(avps, terminal)
	}

	avps = append(avps, unsignedAVP(tgpp.AVPRATType, uint32(r.RATType)), unsignedAVP(AVPULRFlags, uint32(r.Flags)))

	if avps, err = appendEnum(avps, AVPUESRVCCCapability, "UE-SRVCC-Capability", r.UESRVCCCapability); err != nil {
		return nil, err
	}

	visited, err := visitedPLMNAVP(r.VisitedPLMN)
	if err != nil {
		return nil, err
	}

	avps = append(avps, visited)

	if avps, err = appendE164(avps, tgpp.AVPSGSNNumber, "SGSN number", r.SGSNNumber); err != nil {
		return nil, err
	}

	if avps, err = appendEnum(avps, AVPHomogeneousSupportOfIMSVoiceOverPSSession, "Homogeneous-Support-of-IMS-Voice-Over-PS-Sessions", r.HomogeneousIMSVoiceOverPS); err != nil {
		return nil, err
	}

	if len(r.EquivalentPLMNs) > 0 {
		list, err := equivalentPLMNList(r.EquivalentPLMNs)
		if err != nil {
			return nil, err
		}

		avps = append(avps, list)
	}

	if avps, err = appendE164(avps, tgpp.AVPMMENumberForMTSMS, "MME number for MT SMS", r.MMENumberForMTSMS); err != nil {
		return nil, err
	}

	if avps, err = appendEnum(avps, AVPSMSRegisterRequest, "SMS-Register-Request", r.SMSRegisterRequest); err != nil {
		return nil, err
	}

	avps = appendString(avps, AVPSGsMMEIdentity, r.SGsMMEIdentity)
	avps = appendString(avps, AVPCoupledNodeDiameterID, r.CoupledNodeDiameterID)

	return newRequest(env, CommandUpdateLocation, false, avps...)
}

func equivalentPLMNList(plmns []tgpp.PLMN) (diameter.AVP, error) {
	inner := make([]diameter.AVP, 0, len(plmns))

	for _, p := range plmns {
		a, err := visitedPLMNAVP(p)
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, a)
	}

	return groupedAVP(AVPEquivalentPLMNList, inner...), nil
}

func (d decoder) equivalentPLMNList(a diameter.AVP) ([]tgpp.PLMN, error) {
	inner, err := d.group(a, equivalentPLMNListRules)
	if err != nil {
		return nil, err
	}

	ids := diameter.FindAll(inner, AVPVisitedPLMNID, tgpp.VendorID)
	if len(ids) == 0 {
		return nil, diameter.WithinGrouped(a, tgpp.MissingAVP(AVPVisitedPLMNID, tgpp.VendorID, 3))
	}

	plmns := make([]tgpp.PLMN, 0, len(ids))

	for _, id := range ids {
		p, err := parsePLMN(id)
		if err != nil {
			return nil, diameter.WithinGrouped(a, err)
		}

		plmns = append(plmns, p)
	}

	return plmns, nil
}

func CheckUpdateLocation(req *diameter.Message) error {
	return ulrRules.Check(req)
}

func ParseUpdateLocationRequest(req *diameter.Message) (UpdateLocationRequest, error) {
	if err := CheckUpdateLocation(req); err != nil {
		return UpdateLocationRequest{}, err
	}

	var (
		r   UpdateLocationRequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return UpdateLocationRequest{}, err
	}

	r.Features = featuresOf(req)

	if t, ok := req.Find(AVPTerminalInformation, tgpp.VendorID); ok {
		if r.Terminal, err = requestDecoder.terminal(t); err != nil {
			return UpdateLocationRequest{}, err
		}
	}

	ratAVP, _ := req.Find(tgpp.AVPRATType, tgpp.VendorID)

	rat, err := tgpp.Unsigned32(ratAVP)
	if err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.RATType = tgpp.RATType(rat); !r.RATType.ThreeGPP() {
		return UpdateLocationRequest{}, tgpp.InvalidAVP(ratAVP)
	}

	if r.Flags, err = requestFlags(req.AVPs, AVPULRFlags, ulrFlagsDefined); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.UESRVCCCapability, err = optionalEnum[UESRVCCCapability](req.AVPs, AVPUESRVCCCapability, true); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.VisitedPLMN, err = requestVisitedPLMN(req); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.SGSNNumber, err = optionalE164(req.AVPs, tgpp.AVPSGSNNumber); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.HomogeneousIMSVoiceOverPS, err = optionalEnum[IMSVoiceOverPS](req.AVPs, AVPHomogeneousSupportOfIMSVoiceOverPSSession, true); err != nil {
		return UpdateLocationRequest{}, err
	}

	if list, ok := req.Find(AVPEquivalentPLMNList, tgpp.VendorID); ok {
		if r.EquivalentPLMNs, err = requestDecoder.equivalentPLMNList(list); err != nil {
			return UpdateLocationRequest{}, err
		}
	}

	if r.MMENumberForMTSMS, err = optionalE164(req.AVPs, tgpp.AVPMMENumberForMTSMS); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.SMSRegisterRequest, err = optionalEnum[SMSRegisterRequest](req.AVPs, AVPSMSRegisterRequest, true); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.SGsMMEIdentity, err = optionalString(req.AVPs, AVPSGsMMEIdentity, tgpp.VendorID); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.CoupledNodeDiameterID, err = optionalString(req.AVPs, AVPCoupledNodeDiameterID, tgpp.VendorID); err != nil {
		return UpdateLocationRequest{}, err
	}

	return r, nil
}

func validateULASubscription(s SubscriptionData) error {
	switch {
	case s.SubscriberStatus == nil:
		return invalidf("ULA subscription data without Subscriber-Status")
	case *s.SubscriberStatus == SubscriberStatusOperatorDeterminedBarring && s.OperatorDeterminedBarring == nil && s.HPLMNODB == nil:
		return invalidf("OPERATOR_DETERMINED_BARRING without Operator-Determined-Barring or HPLMN-ODB")
	case s.APNConfigurationProfile == nil:
		return nil
	case s.AMBR == nil:
		return invalidf("ULA APN configuration profile without a UE-AMBR")
	}

	if _, ok := s.APNConfigurationProfile.Default(); !ok {
		return invalidf("ULA APN configuration profile without an explicit default APN configuration")
	}

	for _, c := range s.APNConfigurationProfile.Configurations {
		if c.QoS == nil || c.AMBR == nil {
			return invalidf("ULA APN configuration %d without an EPS-Subscribed-QoS-Profile and an AMBR", c.ContextIdentifier)
		}
	}

	return nil
}

func NewUpdateLocationAnswer(req *diameter.Message, id diameter.Identity, u UpdateLocation) (*diameter.Message, error) {
	switch {
	case !u.Flags.Known():
		return nil, invalidf("ULA-Flags with undefined bits %#x", uint32(u.Flags&^ulaFlagsDefined))
	case u.Flags&ULAFlagSeparationIndication == 0:
		return nil, invalidf("ULA-Flags without the Separation Indication")
	}

	ans := newSuccessAnswer(req, id, u.Features)
	ans.AVPs = append(ans.AVPs, unsignedAVP(AVPULAFlags, uint32(u.Flags)))

	if u.Subscription != nil {
		if err := validateULASubscription(*u.Subscription); err != nil {
			return nil, err
		}

		sub, err := u.Subscription.avp()
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, sub)
	}

	ans.AVPs = append(ans.AVPs, resetIDAVPs(u.ResetIDs)...)

	return ans, nil
}

func ParseUpdateLocationAnswer(ans *diameter.Message) (UpdateLocation, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return UpdateLocation{}, err
	}

	u := UpdateLocation{HSS: origin(ans), Features: features, ResetIDs: resetIDs(ans)}

	if u.Flags, err = answerFlags[ULAFlags](ans.AVPs, AVPULAFlags); err != nil {
		return UpdateLocation{}, malformed(err)
	}

	if sub, ok := ans.Find(AVPSubscriptionData, tgpp.VendorID); ok {
		if u.Subscription, err = answerDecoder.subscriptionData(sub); err != nil {
			return UpdateLocation{}, malformed(err)
		}
	}

	return u, nil
}
