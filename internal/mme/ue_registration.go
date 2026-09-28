// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
)

type Registrar interface {
	Register(ctx context.Context, imsi string) (int64, error)
	Confirmed(ctx context.Context, imsi string, version int64) bool
	Purge(imsi string)
	Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context))
}

func (m *MME) RegisterUE(ctx context.Context, ue *UeContext) error {
	if m.Registrations == nil {
		return nil
	}

	version, err := m.Registrations.Register(ctx, ue.IMSI())
	if err != nil {
		return err
	}

	ue.registrationVersion.Store(version)

	return nil
}

func (m *MME) ConfirmRegistration(ctx context.Context, ue *UeContext) error {
	if m.Registrations == nil {
		return nil
	}

	if m.Registrations.Confirmed(ctx, ue.IMSI(), ue.registrationVersion.Load()) {
		return nil
	}

	return m.RegisterUE(ctx, ue)
}

func (m *MME) purgeRegistration(supi etsi.SUPI) {
	if m.Registrations == nil || !supi.IsIMSI() {
		return
	}

	m.Registrations.Purge(supi.IMSI())
}

func (m *MME) HoldsUE(imsi string) bool {
	_, ok := m.LookupUeByIMSI(imsi)
	return ok
}

func (m *MME) ReconcileRegistration(ctx context.Context, imsi string) {
	if m.Registrations == nil {
		return
	}

	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok || ue.EMMState() != EMMRegistered {
		return
	}

	m.Registrations.Reconcile(ctx, imsi, ue.registrationVersion.Load, func(ctx context.Context) {
		m.releaseSuperseded(ctx, ue)
	})
}

func (m *MME) ReconcileRegistrations(ctx context.Context) {
	for _, ue := range m.registeredUEs() {
		if imsi := ue.imsiOrEmpty(); imsi != "" {
			m.ReconcileRegistration(ctx, imsi)
		}
	}
}

func (m *MME) registeredUEs() []*UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ues := make([]*UeContext, 0, len(m.UEs))
	for _, ue := range m.UEs {
		if ue.EMMState() == EMMRegistered {
			ues = append(ues, ue)
		}
	}

	return ues
}

func (m *MME) inHandover(ue *UeContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.handover != nil
}

func (m *MME) releaseSuperseded(ctx context.Context, ue *UeContext) {
	supi := ue.Supi()

	held, ok := m.LookupUeBySupi(supi)
	if !ok || held != ue || ue.EMMState() != EMMRegistered {
		return
	}

	if _, relocating := m.RelocationToFiveGS(ue); relocating {
		return
	}

	if ue.IdleMobilityTo5GSPending() || m.inHandover(ue) {
		return
	}

	m.ReleaseAllSessions(ctx, ue)
	ue.TransitionTo(ctx, EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)

	logger.From(ctx, logger.MmeLog).Info("UE registered on another node; dropping its local EPS registration and PDN connections",
		logger.SUPI(supi.String()))
}
