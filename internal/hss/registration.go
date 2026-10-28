// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"fmt"
	"maps"
	"time"
)

type State int

const (
	NotRegistered State = iota
	Registered
	Unregistered
)

type RegistrationSet string

const (
	MMTel RegistrationSet = "mmtel"
	MCPTT RegistrationSet = "mcptt"
)

type Registration struct {
	ServerName      string
	OriginHost      string
	OriginRealm     string
	PrivateIdentity string
	Sets            map[RegistrationSet]SetRegistration
}

type SetRegistration struct {
	State           State
	AuthPending     bool
	DefaultIdentity string
	ProfileDigest   string
	Emergency       bool
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

	return sameAssignment(a, b) && maps.Equal(a.Sets, b.Sets)
}

func sameAssignment(a, b *Registration) bool {
	return a.ServerName == b.ServerName && a.OriginHost == b.OriginHost && a.OriginRealm == b.OriginRealm && a.PrivateIdentity == b.PrivateIdentity
}

func (r *Registration) set(name RegistrationSet) (SetRegistration, bool) {
	if r == nil {
		return SetRegistration{}, false
	}

	s, ok := r.Sets[name]

	return s, ok
}

func (r *Registration) registered() bool {
	if r == nil {
		return false
	}

	for _, s := range r.Sets {
		if s.State != NotRegistered {
			return true
		}
	}

	return false
}

func (r *Registration) clone() *Registration {
	next := *r
	next.Sets = maps.Clone(r.Sets)

	if next.Sets == nil {
		next.Sets = make(map[RegistrationSet]SetRegistration)
	}

	return &next
}

func (r *Registration) with(name RegistrationSet, s SetRegistration) *Registration {
	next := r.clone()
	next.Sets[name] = s

	return next
}

func (r *Registration) unregistered(names []RegistrationSet) *Registration {
	next := r.clone()

	for _, name := range names {
		s, ok := next.Sets[name]

		switch {
		case !ok:
		case s.State == NotRegistered:
			delete(next.Sets, name)
		default:
			s.State, s.AuthPending = Unregistered, false
			next.Sets[name] = s
		}
	}

	return next
}

func (r *Registration) without(names []RegistrationSet) *Registration {
	if r == nil || len(names) == 0 {
		return nil
	}

	next := r.clone()

	for _, name := range names {
		delete(next.Sets, name)
	}

	if len(next.Sets) == 0 {
		return nil
	}

	return next
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

func assignedElsewhere(current *Registration, serverName string) bool {
	return current != nil && current.ServerName != "" && current.ServerName != serverName
}
