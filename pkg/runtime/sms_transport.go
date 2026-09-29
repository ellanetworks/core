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
	EnableUEReachabilityForSMS(ctx context.Context, imsi string) error
	SendSMS(ctx context.Context, imsi string, payload []byte) error
	SMSSignallingSettled(ctx context.Context, imsi string)
}

type smsTransport struct {
	cores map[smsf.Access]smsCore
}

func newSMSTransport(m *mme.MME, a *amf.AMF) *smsTransport {
	return &smsTransport{cores: map[smsf.Access]smsCore{smsf.AccessEPS: m, smsf.Access5GS: a}}
}

func (t *smsTransport) EnableUEReachability(ctx context.Context, imsi string, access smsf.Access) error {
	core, err := t.core(access)
	if err != nil {
		return err
	}

	return smsTransportError(core.EnableUEReachabilityForSMS(ctx, imsi))
}

func (t *smsTransport) SendSMS(ctx context.Context, imsi string, access smsf.Access, payload []byte) error {
	core, err := t.core(access)
	if err != nil {
		return err
	}

	return smsTransportError(core.SendSMS(ctx, imsi, payload))
}

func (t *smsTransport) SignallingSettled(ctx context.Context, imsi string, access smsf.Access) {
	if core, err := t.core(access); err == nil {
		core.SMSSignallingSettled(ctx, imsi)
	}
}

func (t *smsTransport) core(access smsf.Access) (smsCore, error) {
	core, ok := t.cores[access]
	if !ok {
		return nil, fmt.Errorf("%w: no core for access %s", smsf.ErrUserUnknown, access)
	}

	return core, nil
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

type smsHandler struct {
	smsf   *smsf.SMSF
	access smsf.Access
}

func (h smsHandler) Allowed(ctx context.Context, imsi string) (bool, error) {
	return h.smsf.Allowed(ctx, imsi)
}

func (h smsHandler) Activate(imsi string, owner any) {
	h.smsf.Activate(imsi, h.access, owner)
}

func (h smsHandler) Deactivate(imsi string, owner any) {
	h.smsf.Deactivate(imsi, h.access, owner)
}

func (h smsHandler) Uplink(ctx context.Context, imsi string, payload []byte) {
	h.smsf.Uplink(ctx, imsi, payload)
}

func (h smsHandler) UEReachable(ctx context.Context, imsi string) {
	h.smsf.UEReachable(ctx, imsi)
}

func (h smsHandler) DeliveryFailed(imsi string) {
	h.smsf.DeliveryFailed(imsi)
}

func (h smsHandler) TransactionPending(imsi string) bool {
	return h.smsf.TransactionPending(imsi)
}
