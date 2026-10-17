// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"strings"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type subject struct {
	Subscriber
	domain string
}

func (s *subject) privateIdentity() string {
	return s.IMSI + "@" + s.domain
}

func (s *subject) publicIdentities() []cx.ProfileIdentity {
	var identities []cx.ProfileIdentity

	if s.MSISDN != "" {
		identities = append(identities, cx.ProfileIdentity{Identity: "tel:+" + s.MSISDN})
	}

	return append(identities, cx.ProfileIdentity{Identity: "sip:" + s.privateIdentity(), Barred: true})
}

func (s *subject) authorized() bool {
	for _, identity := range s.publicIdentities() {
		if !identity.Barred {
			return true
		}
	}

	return false
}

func (s *subject) entitled() bool {
	return s.IMSDataNetwork && s.authorized()
}

func (s *subject) userData() ([]byte, error) {
	return cx.MarshalUserData(cx.IMSSubscription{
		PrivateIdentity: s.privateIdentity(),
		ServiceProfiles: []cx.ServiceProfile{{PublicIdentities: s.publicIdentities()}},
	})
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

	for _, impu := range impus {
		owner, err := h.publicIdentityOwner(ctx, impu, domain)
		if err != nil {
			return nil, err
		}

		if sub == nil {
			sub = owner
		}

		if owner.IMSI != sub.IMSI {
			return nil, &cx.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch)}
		}
	}

	if sub == nil {
		return nil, ErrSubscriberUnknown
	}

	return &subject{Subscriber: *sub, domain: strings.ToLower(domain)}, nil
}

func (h *HSS) publicIdentityOwner(ctx context.Context, impu, domain string) (*Subscriber, error) {
	scheme, rest, ok := strings.Cut(impu, ":")
	if !ok {
		return nil, ErrSubscriberUnknown
	}

	switch strings.ToLower(scheme) {
	case "sip":
		imsi, ok := imsiFromPrivateIdentity(rest, domain)
		if !ok {
			return nil, ErrSubscriberUnknown
		}

		return h.store.Subscriber(ctx, imsi)
	case "tel":
		msisdn, ok := strings.CutPrefix(rest, "+")
		if !ok || !isDigits(msisdn, 1, 15) {
			return nil, ErrSubscriberUnknown
		}

		return h.store.SubscriberByMSISDN(ctx, msisdn)
	default:
		return nil, ErrSubscriberUnknown
	}
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
