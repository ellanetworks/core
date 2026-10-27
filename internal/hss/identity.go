// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const mcpttUserPrefix = "mcptt-"

type subject struct {
	Subscriber
	domain    string
	requested []RegistrationSet
}

func (s *subject) privateIdentity() string {
	return s.IMSI + "@" + s.domain
}

func (s *subject) sets() []RegistrationSet {
	if s.MSISDN == "" {
		return []RegistrationSet{MMTel}
	}

	return []RegistrationSet{MMTel, MCPTT}
}

func (s *subject) publicIdentities(set RegistrationSet) []cx.ProfileIdentity {
	switch {
	case set == MMTel && s.MSISDN != "":
		return []cx.ProfileIdentity{
			{Identity: "sip:+" + s.MSISDN + "@" + s.domain},
			{Identity: "tel:+" + s.MSISDN},
			{Identity: "sip:" + s.privateIdentity(), Barred: true},
		}
	case set == MMTel:
		return []cx.ProfileIdentity{{Identity: "sip:" + s.privateIdentity(), Barred: true}}
	case set == MCPTT && s.MSISDN != "":
		return []cx.ProfileIdentity{{Identity: "sip:" + mcpttUserPrefix + s.MSISDN + "@" + s.domain}}
	default:
		return nil
	}
}

func (s *subject) requestedSet() RegistrationSet {
	if len(s.requested) == 0 {
		return MMTel
	}

	return s.requested[0]
}

func (s *subject) authorized(set RegistrationSet) bool {
	return s.defaultIdentity(set) != ""
}

func (s *subject) entitled(set RegistrationSet) bool {
	return s.IMSDataNetwork && s.authorized(set)
}

func hasUnregisteredTerminatingServices(ifcs []cx.InitialFilterCriteria) bool {
	return slices.ContainsFunc(ifcs, func(ifc cx.InitialFilterCriteria) bool {
		if ifc.ProfilePart != nil && *ifc.ProfilePart == cx.ProfilePartRegistered {
			return false
		}

		return ifc.Trigger == nil || mayMatchTerminatingUnregistered(*ifc.Trigger)
	})
}

func mayMatchTerminatingUnregistered(t cx.TriggerPoint) bool {
	groups := make(map[int32][]bool)

	for _, spt := range t.ServicePointTriggers {
		possible := sptMayMatchTerminatingUnregistered(spt)
		for _, g := range spt.Groups {
			groups[g] = append(groups[g], possible)
		}
	}

	for _, possible := range groups {
		if t.ConjunctiveNormalForm && !slices.Contains(possible, true) {
			return false
		}

		if !t.ConjunctiveNormalForm && !slices.Contains(possible, false) {
			return true
		}
	}

	return t.ConjunctiveNormalForm
}

func sptMayMatchTerminatingUnregistered(spt cx.ServicePointTrigger) bool {
	switch {
	case spt.SessionCase != nil:
		return (*spt.SessionCase == cx.SessionCaseTerminatingUnregistered) != spt.Negated
	case spt.Method != nil && *spt.Method == "REGISTER":
		return spt.Negated
	default:
		return true
	}
}

func (s *subject) defaultIdentity(set RegistrationSet) string {
	for _, identity := range s.publicIdentities(set) {
		if !identity.Barred {
			return identity.Identity
		}
	}

	return ""
}

type profile struct {
	privateIdentity string
	defaultIdentity string
	userData        []byte
	digest          string
}

func (h *HSS) profileOf(ctx context.Context, s *subject, set RegistrationSet) (*profile, error) {
	ifcs, err := h.store.FilterCriteria(ctx, s.IMSI, set)
	if err != nil {
		return nil, err
	}

	userData, err := cx.MarshalUserData(cx.IMSSubscription{
		PrivateIdentity: s.privateIdentity(),
		ServiceProfiles: []cx.ServiceProfile{{PublicIdentities: s.publicIdentities(set), InitialFilterCriteria: ifcs}},
	})
	if err != nil {
		return nil, err
	}

	return &profile{
		privateIdentity: s.privateIdentity(),
		defaultIdentity: s.defaultIdentity(set),
		userData:        userData,
		digest:          digest(userData),
	}, nil
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:])
}

func (h *HSS) resolve(ctx context.Context, impi string, impus ...string) (*subject, error) {
	domain, err := h.store.IMSDomain(ctx)
	if err != nil {
		return nil, err
	}

	var sub *Subscriber

	if impi != "" {
		imsi, ok := imsiFromPrivateIdentity(impi, domain)
		if !ok {
			return nil, ErrSubscriberUnknown
		}

		if sub, err = h.store.Subscriber(ctx, imsi); err != nil {
			return nil, err
		}
	}

	var requested []RegistrationSet

	for _, impu := range impus {
		owner, set, err := h.publicIdentityOwner(ctx, impu, domain)
		if err != nil {
			return nil, err
		}

		if sub == nil {
			sub = owner
		}

		if owner.IMSI != sub.IMSI {
			return nil, &cx.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch)}
		}

		if !slices.Contains(requested, set) {
			requested = append(requested, set)
		}
	}

	if sub == nil {
		return nil, ErrSubscriberUnknown
	}

	return &subject{Subscriber: *sub, domain: strings.ToLower(domain), requested: requested}, nil
}

func (h *HSS) publicIdentityOwner(ctx context.Context, impu, domain string) (*Subscriber, RegistrationSet, error) {
	scheme, rest, ok := strings.Cut(impu, ":")
	if !ok {
		return nil, "", ErrSubscriberUnknown
	}

	var (
		sub *Subscriber
		set = MMTel
		err error
	)

	switch strings.ToLower(scheme) {
	case "sip":
		user, host, ok := strings.Cut(rest, "@")
		if !ok || !strings.EqualFold(host, domain) {
			return nil, "", ErrSubscriberUnknown
		}

		switch {
		case strings.HasPrefix(user, "+"):
			sub, err = h.subscriberByMSISDN(ctx, user[1:])
		case strings.HasPrefix(user, mcpttUserPrefix):
			set = MCPTT
			sub, err = h.subscriberByMSISDN(ctx, user[len(mcpttUserPrefix):])
		case isDigits(user, 6, 15):
			sub, err = h.store.Subscriber(ctx, user)
		default:
			err = ErrSubscriberUnknown
		}
	case "tel":
		msisdn, ok := strings.CutPrefix(rest, "+")
		if !ok {
			return nil, "", ErrSubscriberUnknown
		}

		sub, err = h.subscriberByMSISDN(ctx, msisdn)
	default:
		err = ErrSubscriberUnknown
	}

	if err != nil {
		return nil, "", err
	}

	return sub, set, nil
}

func identitySet(impu string) RegistrationSet {
	scheme, rest, ok := strings.Cut(impu, ":")
	if ok && strings.EqualFold(scheme, "sip") && strings.HasPrefix(rest, mcpttUserPrefix) {
		return MCPTT
	}

	return MMTel
}

func (h *HSS) subscriberByMSISDN(ctx context.Context, msisdn string) (*Subscriber, error) {
	if !isDigits(msisdn, 1, 15) {
		return nil, ErrSubscriberUnknown
	}

	return h.store.SubscriberByMSISDN(ctx, msisdn)
}

func imsiFromPrivateIdentity(impi, domain string) (string, bool) {
	user, host, ok := strings.Cut(impi, "@")
	if !ok || !strings.EqualFold(host, domain) || !isDigits(user, 6, 15) {
		return "", false
	}

	return user, true
}

func isDigits(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
