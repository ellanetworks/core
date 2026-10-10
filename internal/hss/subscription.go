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
	Identity  string
	Barred    bool
	UserState UserState
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
	state := userState(reg)

	out := &IMSSubscription{PrivateIdentity: s.privateIdentity()}

	if reg != nil && state != UserNotRegistered {
		out.SCSCFName = reg.ServerName
	}

	for _, identity := range s.publicIdentities() {
		out.PublicIdentities = append(out.PublicIdentities, PublicIdentity{Identity: identity.Identity, Barred: identity.Barred, UserState: state})
	}

	return out, nil
}

func userState(reg *Registration) UserState {
	switch {
	case reg == nil:
		return UserNotRegistered
	case reg.State == Registered:
		return UserRegistered
	case reg.State == Unregistered:
		return UserRegisteredUnregServices
	case reg.AuthPending:
		return UserAuthenticationPending
	default:
		return UserNotRegistered
	}
}
