// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/diameter"
)

type State int

const (
	NotRegistered State = iota
	Registered
	Unregistered
)

type Registration struct {
	State       State
	ServerName  string
	AuthPending bool
	OriginHost  string
	OriginRealm string
	UpdatedAt   int64
}

type RegistrationStore interface {
	Registration(ctx context.Context, imsi string) (*Registration, error)
	CompareAndSwapRegistration(ctx context.Context, imsi string, expected, next *Registration) (*Registration, bool, error)
}

const maxRegistrationAttempts = 8

func sameRegistration(a, b *Registration) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return a.State == b.State &&
		a.ServerName == b.ServerName &&
		a.AuthPending == b.AuthPending &&
		a.OriginHost == b.OriginHost &&
		a.OriginRealm == b.OriginRealm
}

func (h *HSS) updateRegistration(ctx context.Context, imsi string, next func(current *Registration) (*Registration, error)) error {
	current, err := h.store.Registration(ctx, imsi)
	if err != nil {
		return err
	}

	for range maxRegistrationAttempts {
		want, err := next(current)
		if err != nil {
			return err
		}

		if sameRegistration(current, want) {
			return nil
		}

		if want != nil {
			stamped := *want
			stamped.UpdatedAt = h.now().Unix()
			want = &stamped
		}

		stored, swapped, err := h.store.CompareAndSwapRegistration(ctx, imsi, current, want)
		if err != nil {
			return err
		}

		if swapped {
			return nil
		}

		current = stored
	}

	return fmt.Errorf("IMS registration of subscriber %s contended beyond %d attempts", imsi, maxRegistrationAttempts)
}

func origin(req *diameter.Message) (host, realm string) {
	if a, ok := req.Find(diameter.AVPOriginHost, 0); ok {
		host = a.UTF8String()
	}

	if a, ok := req.Find(diameter.AVPOriginRealm, 0); ok {
		realm = a.UTF8String()
	}

	return host, realm
}

func assignedElsewhere(current *Registration, serverName string) bool {
	return current != nil && current.ServerName != "" && current.ServerName != serverName
}
