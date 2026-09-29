// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/smsf"
)

type smsCore interface {
	SMSRoute(imsi string) (granted, connected bool)
	EnableUEReachabilityForSMS(ctx context.Context, imsi string) error
	SendSMS(ctx context.Context, imsi string, payload []byte) error
	SMSSignallingSettled(ctx context.Context, imsi string)
}

type smsTransport struct {
	cores []smsCore
}

func newSMSTransport(a *amf.AMF, m *mme.MME) *smsTransport {
	return &smsTransport{cores: []smsCore{a, m}}
}

func (t *smsTransport) EnableUEReachability(ctx context.Context, imsi string) error {
	return smsTransportError(t.route(imsi).EnableUEReachabilityForSMS(ctx, imsi))
}

func (t *smsTransport) SendSMS(ctx context.Context, imsi string, payload []byte) error {
	return smsTransportError(t.route(imsi).SendSMS(ctx, imsi, payload))
}

func (t *smsTransport) SignallingSettled(ctx context.Context, imsi string) {
	for _, core := range t.cores {
		core.SMSSignallingSettled(ctx, imsi)
	}
}

func (t *smsTransport) route(imsi string) smsCore {
	var fallback smsCore

	for _, core := range t.cores {
		granted, connected := core.SMSRoute(imsi)

		switch {
		case granted && connected:
			return core
		case granted && fallback == nil:
			fallback = core
		}
	}

	if fallback != nil {
		return fallback
	}

	return t.cores[0]
}

func smsTransportError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mme.ErrSMSUENotRegistered), errors.Is(err, amf.ErrSMSUENotRegistered):
		return fmt.Errorf("%w: %w", smsf.ErrUserUnknown, err)
	case errors.Is(err, mme.ErrSMSNotAllowed), errors.Is(err, amf.ErrSMSNotAllowed):
		return fmt.Errorf("%w: %w", smsf.ErrNotRegisteredForSMS, err)
	case errors.Is(err, mme.ErrSMSUEUnreachable), errors.Is(err, amf.ErrSMSUEUnreachable):
		return fmt.Errorf("%w: %w", &smsf.AbsentError{Diagnostic: tgpp.AbsentUserNoPagingResponseMSC}, err)
	default:
		return err
	}
}
