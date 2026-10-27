// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type InsertSubscriberDataRequest struct {
	IMSI         string
	Features     Features
	Subscription SubscriptionData
	Flags        IDRFlags
	ResetIDs     []ResetID
}

type InsertSubscriberData struct {
	Features                Features
	IMSVoiceOverPSSupported *IMSVoiceOverPS
	LastUEActivityTime      time.Time
	RATType                 *tgpp.RATType
	Flags                   IDAFlags
	LocalTimeZone           *LocalTimeZone
}

type LocalTimeZone struct {
	TimeZone       string
	DaylightSaving DaylightSavingTime
}

var (
	idrRules = requestRules(true, userNameRule.With(diameter.Rules{
		vendorKey(AVPSubscriptionData): {Required: true},
		vendorKey(AVPIDRFlags):         {},
		vendorKey(AVPResetID):          {Multiple: true},
	}))
	localTimeZoneRules = diameter.Rules{
		vendorKey(AVPTimeZone):           {},
		vendorKey(AVPDaylightSavingTime): {},
	}
)

func NewInsertSubscriberDataRequest(env tgpp.Envelope, r InsertSubscriberDataRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	sub, err := r.Subscription.avp()
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)
	avps = append(avps, sub)

	if avps, err = appendFlags(avps, AVPIDRFlags, "IDR-Flags", r.Flags); err != nil {
		return nil, err
	}

	avps = append(avps, resetIDAVPs(r.ResetIDs)...)

	return newRequest(env, CommandInsertSubscriberData, true, avps...)
}

func CheckInsertSubscriberData(req *diameter.Message) error {
	return idrRules.Check(req)
}

func ParseInsertSubscriberDataRequest(req *diameter.Message) (InsertSubscriberDataRequest, error) {
	if err := CheckInsertSubscriberData(req); err != nil {
		return InsertSubscriberDataRequest{}, err
	}

	var (
		r   InsertSubscriberDataRequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return InsertSubscriberDataRequest{}, err
	}

	r.Features = featuresOf(req)

	subAVP, _ := req.Find(AVPSubscriptionData, tgpp.VendorID)

	sub, err := requestDecoder.subscriptionData(subAVP)
	if err != nil {
		return InsertSubscriberDataRequest{}, err
	}

	r.Subscription = *sub

	if r.Flags, err = requestFlags(req.AVPs, AVPIDRFlags, idrFlagsDefined); err != nil {
		return InsertSubscriberDataRequest{}, err
	}

	r.ResetIDs = resetIDs(req)

	return r, nil
}

func (l LocalTimeZone) avp() (diameter.AVP, error) {
	if l.TimeZone == "" {
		return diameter.AVP{}, invalidf("local time zone without a time zone")
	}

	if !l.DaylightSaving.Known() {
		return diameter.AVP{}, invalidf("Daylight-Saving-Time %d", l.DaylightSaving)
	}

	return groupedAVP(AVPLocalTimeZone,
		stringAVP(AVPTimeZone, l.TimeZone),
		unsignedAVP(AVPDaylightSavingTime, uint32(l.DaylightSaving)),
	), nil
}

func (d decoder) localTimeZone(a diameter.AVP) (*LocalTimeZone, error) {
	inner, err := d.group(a, localTimeZoneRules)
	if err != nil {
		return nil, err
	}

	tz, ok := diameter.Find(inner, AVPTimeZone, tgpp.VendorID)

	switch {
	case !ok:
		return nil, diameter.WithinGrouped(a, tgpp.MissingAVP(AVPTimeZone, tgpp.VendorID, 0))
	case len(tz.Data) == 0:
		return nil, diameter.WithinGrouped(a, tgpp.InvalidAVP(tz))
	}

	dst, err := requiredEnum[DaylightSavingTime](inner, AVPDaylightSavingTime, d.strict)
	if err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	return &LocalTimeZone{TimeZone: tz.UTF8String(), DaylightSaving: dst}, nil
}

func NewInsertSubscriberDataAnswer(req *diameter.Message, id diameter.Identity, a InsertSubscriberData) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, a.Features)

	var err error

	if ans.AVPs, err = appendEnum(ans.AVPs, AVPIMSVoiceOverPSSessionsSupported, "IMS-Voice-Over-PS-Sessions-Supported", a.IMSVoiceOverPSSupported); err != nil {
		return nil, err
	}

	if !a.LastUEActivityTime.IsZero() {
		if !diameter.ValidTime(a.LastUEActivityTime) {
			return nil, invalidf("Last-UE-Activity-Time %s", a.LastUEActivityTime)
		}

		ans.AVPs = append(ans.AVPs, diameter.Time(AVPLastUEActivityTime, avpFlags(AVPLastUEActivityTime), tgpp.VendorID, a.LastUEActivityTime))
	}

	if a.RATType != nil {
		if !a.RATType.ThreeGPP() {
			return nil, invalidf("RAT-Type %s is not a 3GPP radio access technology", *a.RATType)
		}

		ans.AVPs = append(ans.AVPs, unsignedAVP(tgpp.AVPRATType, uint32(*a.RATType)))
	}

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPIDAFlags, "IDA-Flags", a.Flags); err != nil {
		return nil, err
	}

	if a.LocalTimeZone != nil {
		tz, err := a.LocalTimeZone.avp()
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, tz)
	}

	return ans, nil
}

func ParseInsertSubscriberDataAnswer(ans *diameter.Message) (InsertSubscriberData, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return InsertSubscriberData{}, err
	}

	a := InsertSubscriberData{Features: features}

	if a.IMSVoiceOverPSSupported, err = optionalEnum[IMSVoiceOverPS](ans.AVPs, AVPIMSVoiceOverPSSessionsSupported, false); err != nil {
		return InsertSubscriberData{}, malformed(err)
	}

	if t, ok := ans.Find(AVPLastUEActivityTime, tgpp.VendorID); ok {
		if a.LastUEActivityTime, err = t.Time(); err != nil {
			return InsertSubscriberData{}, malformed(err)
		}
	}

	if rat, ok, err := optionalUnsigned(ans.AVPs, tgpp.AVPRATType); err != nil {
		return InsertSubscriberData{}, malformed(err)
	} else if ok {
		t := tgpp.RATType(rat)
		a.RATType = &t
	}

	if a.Flags, err = answerFlags[IDAFlags](ans.AVPs, AVPIDAFlags); err != nil {
		return InsertSubscriberData{}, malformed(err)
	}

	if tz, ok := ans.Find(AVPLocalTimeZone, tgpp.VendorID); ok {
		if a.LocalTimeZone, err = answerDecoder.localTimeZone(tz); err != nil {
			return InsertSubscriberData{}, malformed(err)
		}
	}

	return a, nil
}
