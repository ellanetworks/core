// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"fmt"
	"time"

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

const (
	maxRegistrationAttempts = 8
	commitTimeout           = 4 * time.Second
)

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
	return bounded(ctx, func(ctx context.Context) error { return h.swapRegistration(ctx, imsi, next) })
}

func bounded(ctx context.Context, f func(ctx context.Context) error) error {
	done := make(chan error, 1)

	go func() { done <- f(context.WithoutCancel(ctx)) }()

	timer := time.NewTimer(commitTimeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("%w: not committed within %s", ErrUnavailable, commitTimeout)
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
	}
}

func (h *HSS) swapRegistration(ctx context.Context, imsi string, next func(current *Registration) (*Registration, error)) error {
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

	return fmt.Errorf("%w: IMS registration of subscriber %s contended beyond %d attempts", ErrUnavailable, imsi, maxRegistrationAttempts)
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
