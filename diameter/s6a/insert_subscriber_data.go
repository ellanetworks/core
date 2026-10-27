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
	ResetIDs     [][]byte
}

type InsertSubscriberData struct {
	Features                Features
	IMSVoiceOverPSSupported *IMSVoiceOverPS
	LastUEActivityTime      time.Time
	RATType                 *tgpp.RATType
	Flags                   IDAFlags
	LocalTimeZone           *LocalTimeZone
}

var idrRules = requestRules(true, userNameRule.With(diameter.Rules{
	vendorKey(AVPSubscriptionData): {},
	vendorKey(AVPIDRFlags):         {},
	vendorKey(AVPResetID):          {Multiple: true},
}))

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

	if avps, err = appendFlags(avps, AVPIDRFlags, "IDR-Flags", r.Flags, idrFlagsDefined); err != nil {
		return nil, err
	}

	avps = append(avps, resetIDAVPs(r.ResetIDs)...)

	return newRequest(env, CommandInsertSubscriberData, true, avps...)
}

func CheckInsertSubscriberData(req *diameter.Message) error {
	if err := idrRules.Check(req); err != nil {
		return err
	}

	if _, ok := req.Find(AVPSubscriptionData, tgpp.VendorID); !ok {
		return tgpp.MissingAVP(AVPSubscriptionData, tgpp.VendorID, 0)
	}

	return nil
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

	sub, err := parseSubscriptionData(subAVP)
	if err != nil {
		return InsertSubscriberDataRequest{}, err
	}

	r.Subscription = *sub

	if r.Flags, err = optionalFlags(req.AVPs, AVPIDRFlags, idrFlagsDefined); err != nil {
		return InsertSubscriberDataRequest{}, err
	}

	r.ResetIDs = resetIDs(req)

	return r, nil
}

func NewInsertSubscriberDataAnswer(req *diameter.Message, id diameter.Identity, a InsertSubscriberData) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, a.Features)

	var err error

	if ans.AVPs, err = appendEnum(ans.AVPs, AVPIMSVoiceOverPSSessionsSupported, "IMS-Voice-Over-PS-Sessions-Supported", a.IMSVoiceOverPSSupported); err != nil {
		return nil, err
	}

	if !a.LastUEActivityTime.IsZero() {
		ans.AVPs = append(ans.AVPs, diameter.Time(AVPLastUEActivityTime, 0, tgpp.VendorID, a.LastUEActivityTime))
	}

	if a.RATType != nil {
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(tgpp.AVPRATType, 0, tgpp.VendorID, uint32(*a.RATType)))
	}

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPIDAFlags, "IDA-Flags", a.Flags, idaFlagsDefined); err != nil {
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

	if a.IMSVoiceOverPSSupported, err = optionalEnum[IMSVoiceOverPS](ans.AVPs, AVPIMSVoiceOverPSSessionsSupported); err != nil {
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

	if a.Flags, err = optionalFlags(ans.AVPs, AVPIDAFlags, idaFlagsDefined); err != nil {
		return InsertSubscriberData{}, malformed(err)
	}

	if tz, ok := ans.Find(AVPLocalTimeZone, tgpp.VendorID); ok {
		if a.LocalTimeZone, err = parseLocalTimeZone(tz); err != nil {
			return InsertSubscriberData{}, malformed(err)
		}
	}

	return a, nil
}
