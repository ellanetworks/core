// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const maxUserIDDigits = 15

type ResetID string

type ResetRequest struct {
	Features     Features
	UserIDs      []string
	ResetIDs     []ResetID
	Subscription *SubscriptionData
	Deletion     *SubscriptionDataDeletion
}

type Reset struct {
	Features Features
}

func (r ResetRequest) Affects(imsi string, stored []ResetID) bool {
	if len(r.ResetIDs) > 0 {
		return slices.ContainsFunc(r.ResetIDs, func(id ResetID) bool { return slices.Contains(stored, id) })
	}

	if len(r.UserIDs) == 0 {
		return true
	}

	return slices.ContainsFunc(r.UserIDs, func(prefix string) bool { return strings.HasPrefix(imsi, prefix) })
}

var rsrRules = requestRules(true, diameter.Rules{
	vendorKey(AVPUserID):                   {Multiple: true},
	vendorKey(AVPResetID):                  {Multiple: true},
	vendorKey(AVPSubscriptionData):         {},
	vendorKey(AVPSubscriptionDataDeletion): {},
})

func NewResetRequest(env tgpp.Envelope, r ResetRequest) (*diameter.Message, error) {
	switch {
	case r.Subscription != nil && r.Deletion != nil:
		return nil, invalidf("reset with both Subscription-Data and Subscription-Data-Deletion")
	case (r.Subscription != nil || r.Deletion != nil) && len(r.ResetIDs) == 0:
		return nil, invalidf("reset updating shared subscription data without a Reset-ID")
	}

	avps := r.Features.avps()

	for _, id := range r.UserIDs {
		if !tgpp.Digits(id, 1, maxUserIDDigits) {
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

	if r.Deletion != nil {
		inner, err := r.Deletion.avps()
		if err != nil {
			return nil, err
		}

		avps = append(avps, groupedAVP(AVPSubscriptionDataDeletion, inner...))
	}

	return newRequest(env, CommandReset, true, avps...)
}

func CheckReset(req *diameter.Message) error {
	if err := rsrRules.Check(req); err != nil {
		return err
	}

	sub, hasSub := req.Find(AVPSubscriptionData, tgpp.VendorID)
	del, hasDel := req.Find(AVPSubscriptionDataDeletion, tgpp.VendorID)
	_, hasResetID := req.Find(AVPResetID, tgpp.VendorID)

	switch {
	case hasSub && hasDel:
		return diameter.NewAVPError(diameter.ResultContradictingAVPs, sub, del)
	case (hasSub || hasDel) && !hasResetID:
		return tgpp.MissingAVP(AVPResetID, tgpp.VendorID, 0)
	}

	return nil
}

func ParseResetRequest(req *diameter.Message) (ResetRequest, error) {
	if err := CheckReset(req); err != nil {
		return ResetRequest{}, err
	}

	r := ResetRequest{Features: featuresOf(req), ResetIDs: resetIDs(req)}

	for _, a := range diameter.FindAll(req.AVPs, AVPUserID, tgpp.VendorID) {
		if !tgpp.Digits(a.UTF8String(), 1, maxUserIDDigits) {
			return ResetRequest{}, tgpp.InvalidAVP(a)
		}

		r.UserIDs = append(r.UserIDs, a.UTF8String())
	}

	var err error

	if sub, ok := req.Find(AVPSubscriptionData, tgpp.VendorID); ok {
		if r.Subscription, err = requestDecoder.subscriptionData(sub); err != nil {
			return ResetRequest{}, err
		}
	}

	if del, ok := req.Find(AVPSubscriptionDataDeletion, tgpp.VendorID); ok {
		inner, err := subscriptionDataDeletionRules.CheckGrouped(del)
		if err != nil {
			return ResetRequest{}, err
		}

		d, err := parseSubscriptionDataDeletion(inner)
		if err != nil {
			return ResetRequest{}, diameter.WithinGrouped(del, err)
		}

		r.Deletion = &d
	}

	return r, nil
}

func NewResetAnswer(req *diameter.Message, id diameter.Identity, r Reset) (*diameter.Message, error) {
	return newSuccessAnswer(req, id, r.Features), nil
}

func ParseResetAnswer(ans *diameter.Message) (Reset, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return Reset{}, err
	}

	return Reset{Features: features}, nil
}

func resetIDAVPs(ids []ResetID) []diameter.AVP {
	avps := make([]diameter.AVP, 0, len(ids))

	for _, id := range ids {
		avps = append(avps, stringAVP(AVPResetID, string(id)))
	}

	return avps
}

func resetIDs(m *diameter.Message) []ResetID {
	var ids []ResetID

	for _, a := range diameter.FindAll(m.AVPs, AVPResetID, tgpp.VendorID) {
		ids = append(ids, ResetID(a.Data))
	}

	return ids
}
