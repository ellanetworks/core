// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const maxUserIDDigits = 15

type ResetRequest struct {
	Features     Features
	UserIDs      []string
	ResetIDs     [][]byte
	Subscription *SubscriptionData
}

func (r ResetRequest) Matches(imsi string) bool {
	if len(r.UserIDs) == 0 {
		return true
	}

	for _, prefix := range r.UserIDs {
		if strings.HasPrefix(imsi, prefix) {
			return true
		}
	}

	return false
}

var rsrRules = requestRules(true, diameter.Rules{
	vendorKey(AVPUserID):                   {Multiple: true},
	vendorKey(AVPResetID):                  {Multiple: true},
	vendorKey(AVPSubscriptionData):         {},
	vendorKey(AVPSubscriptionDataDeletion): {},
})

func NewResetRequest(env tgpp.Envelope, r ResetRequest) (*diameter.Message, error) {
	avps := r.Features.avps()

	for _, id := range r.UserIDs {
		if !digits(id, 1, maxUserIDDigits) {
			return nil, invalidf("User-Id %q", id)
		}

		avps = append(avps, stringAVP(AVPUserID, id))
	}

	avps = append(avps, resetIDAVPs(r.ResetIDs)...)

	if r.Subscription != nil {
		sub, err := r.Subscription.avp()
		if err != nil {
			return nil, err
		}

		avps = append(avps, sub)
	}

	return newRequest(env, CommandReset, true, avps...)
}

func CheckReset(req *diameter.Message) error {
	return rsrRules.Check(req)
}

func ParseResetRequest(req *diameter.Message) (ResetRequest, error) {
	if err := CheckReset(req); err != nil {
		return ResetRequest{}, err
	}

	r := ResetRequest{Features: featuresOf(req), ResetIDs: resetIDs(req)}

	for _, a := range diameter.FindAll(req.AVPs, AVPUserID, tgpp.VendorID) {
		if !digits(a.UTF8String(), 1, maxUserIDDigits) {
			return ResetRequest{}, tgpp.InvalidAVP(a)
		}

		r.UserIDs = append(r.UserIDs, a.UTF8String())
	}

	if sub, ok := req.Find(AVPSubscriptionData, tgpp.VendorID); ok {
		var err error
		if r.Subscription, err = parseSubscriptionData(sub); err != nil {
			return ResetRequest{}, err
		}
	}

	return r, nil
}

func NewResetAnswer(req *diameter.Message, id diameter.Identity, features Features) *diameter.Message {
	return newSuccessAnswer(req, id, features)
}

func ParseResetAnswer(ans *diameter.Message) (Features, error) {
	return parseAnswer(ans)
}
