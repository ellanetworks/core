// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"

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
	Features     Features
	Flags        ULAFlags
	Subscription *SubscriptionData
	ResetIDs     [][]byte
}

var ulrRules = requestRules(false, userNameRule.With(diameter.Rules{
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

func NewUpdateLocationRequest(env tgpp.Envelope, r UpdateLocationRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)

	if r.Terminal != nil {
		terminal, err := r.Terminal.avp()
		if err != nil {
			return nil, err
		}

		avps = append(avps, terminal)
	}

	avps = append(avps, diameter.Unsigned32(tgpp.AVPRATType, 0, tgpp.VendorID, uint32(r.RATType)))

	if r.Flags&^ulrFlagsDefined != 0 {
		return nil, invalidf("ULR-Flags with undefined bits %#x", uint32(r.Flags&^ulrFlagsDefined))
	}

	avps = append(avps, unsignedAVP(AVPULRFlags, uint32(r.Flags)))

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

func parseEquivalentPLMNList(a diameter.AVP) ([]tgpp.PLMN, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	ids := diameter.FindAll(inner, AVPVisitedPLMNID, tgpp.VendorID)
	if len(ids) == 0 {
		return nil, tgpp.MissingAVP(AVPVisitedPLMNID, tgpp.VendorID, 3)
	}

	plmns := make([]tgpp.PLMN, 0, len(ids))

	for _, id := range ids {
		p, err := tgpp.DecodePLMN(id.Data)
		if err != nil {
			return nil, tgpp.InvalidAVP(id)
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
		if r.Terminal, err = parseTerminalInformation(t); err != nil {
			return UpdateLocationRequest{}, err
		}
	}

	rat, _, err := optionalUnsigned(req.AVPs, tgpp.AVPRATType)
	if err != nil {
		return UpdateLocationRequest{}, err
	}

	r.RATType = tgpp.RATType(rat)

	if r.Flags, err = optionalFlags(req.AVPs, AVPULRFlags, ulrFlagsDefined); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.UESRVCCCapability, err = optionalEnum[UESRVCCCapability](req.AVPs, AVPUESRVCCCapability); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.VisitedPLMN, err = requestVisitedPLMN(req); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.SGSNNumber, err = optionalE164(req.AVPs, tgpp.AVPSGSNNumber); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.HomogeneousIMSVoiceOverPS, err = optionalEnum[IMSVoiceOverPS](req.AVPs, AVPHomogeneousSupportOfIMSVoiceOverPSSession); err != nil {
		return UpdateLocationRequest{}, err
	}

	if list, ok := req.Find(AVPEquivalentPLMNList, tgpp.VendorID); ok {
		if r.EquivalentPLMNs, err = parseEquivalentPLMNList(list); err != nil {
			return UpdateLocationRequest{}, err
		}
	}

	if r.MMENumberForMTSMS, err = optionalE164(req.AVPs, tgpp.AVPMMENumberForMTSMS); err != nil {
		return UpdateLocationRequest{}, err
	}

	if r.SMSRegisterRequest, err = optionalEnum[SMSRegisterRequest](req.AVPs, AVPSMSRegisterRequest); err != nil {
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

func NewUpdateLocationAnswer(req *diameter.Message, id diameter.Identity, u UpdateLocation) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, u.Features)

	var err error

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPULAFlags, "ULA-Flags", u.Flags, ulaFlagsDefined); err != nil {
		return nil, err
	}

	if u.Subscription != nil {
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

	u := UpdateLocation{Features: features, ResetIDs: resetIDs(ans)}

	if u.Flags, err = optionalFlags(ans.AVPs, AVPULAFlags, ulaFlagsDefined); err != nil {
		return UpdateLocation{}, malformed(err)
	}

	if sub, ok := ans.Find(AVPSubscriptionData, tgpp.VendorID); ok {
		if u.Subscription, err = parseSubscriptionData(sub); err != nil {
			return UpdateLocation{}, malformed(err)
		}
	}

	return u, nil
}

func resetIDAVPs(ids [][]byte) []diameter.AVP {
	avps := make([]diameter.AVP, 0, len(ids))

	for _, id := range ids {
		avps = append(avps, octetsAVP(AVPResetID, id))
	}

	return avps
}

func resetIDs(m *diameter.Message) [][]byte {
	var ids [][]byte

	for _, a := range diameter.FindAll(m.AVPs, AVPResetID, tgpp.VendorID) {
		ids = append(ids, bytes.Clone(a.Data))
	}

	return ids
}
