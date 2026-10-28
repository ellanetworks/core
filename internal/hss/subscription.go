// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"strings"
)

type UserState string

const (
	UserRegistered              UserState = "registered"
	UserRegisteredUnregServices UserState = "registered_unreg_services"
	UserAuthenticationPending   UserState = "authentication_pending"
	UserNotRegistered           UserState = "not_registered"
)

type PublicIdentity struct {
	Identity        string
	Barred          bool
	RegistrationSet RegistrationSet
	UserState       UserState
}

type IMSSubscription struct {
	PrivateIdentity  string
	SCSCFName        string
	PublicIdentities []PublicIdentity
}

func (h *HSS) Subscription(ctx context.Context, imsi string) (*IMSSubscription, error) {
	sub, err := h.store.Subscriber(ctx, imsi)
	if err != nil {
		return nil, err
	}

	reg, err := h.store.Registration(ctx, imsi)
	if err != nil {
		return nil, err
	}

	if !sub.IMSDataNetwork && reg == nil {
		return nil, nil
	}

	domain, err := h.store.IMSDomain(ctx)
	if err != nil {
		return nil, err
	}

	s := subject{Subscriber: *sub, domain: strings.ToLower(domain)}

	out := &IMSSubscription{PrivateIdentity: s.privateIdentity()}

	for _, set := range s.sets() {
		state := userState(reg, set)

		if state != UserNotRegistered {
			out.SCSCFName = reg.ServerName
		}

		for _, identity := range s.publicIdentities(set) {
			out.PublicIdentities = append(out.PublicIdentities, PublicIdentity{Identity: identity.Identity, Barred: identity.Barred, RegistrationSet: set, UserState: state})
		}
	}

	return out, nil
}

func userState(reg *Registration, set RegistrationSet) UserState {
	s, ok := reg.set(set)

	switch {
	case !ok:
		return UserNotRegistered
	case s.State == Registered:
		return UserRegistered
	case s.State == Unregistered:
		return UserRegisteredUnregServices
	case s.AuthPending:
		return UserAuthenticationPending
	default:
		return UserNotRegistered
	}
}
