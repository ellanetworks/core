// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
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
	return t.route(imsi).EnableUEReachabilityForSMS(ctx, imsi)
}

func (t *smsTransport) SendSMS(ctx context.Context, imsi string, payload []byte) error {
	return t.route(imsi).SendSMS(ctx, imsi, payload)
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
