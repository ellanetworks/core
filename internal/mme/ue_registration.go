// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
)

type Registrar interface {
	Register(ctx context.Context, imsi string) error
	Purge(ctx context.Context, imsi string, stillAbsent func() bool)
	Superseded(ctx context.Context, imsi string, registeredAt int64) bool
}

func (m *MME) RegisterUE(ctx context.Context, ue *UeContext) error {
	if m.Registrations == nil {
		return nil
	}

	start := time.Now().UnixMilli()

	if err := m.Registrations.Register(ctx, ue.IMSI()); err != nil {
		return err
	}

	ue.registeredAt.Store(start)

	return nil
}

func (m *MME) purgeRegistrationAsync(supi etsi.SUPI) {
	if m.Registrations == nil || !supi.IsIMSI() {
		return
	}

	go m.Registrations.Purge(context.Background(), supi.IMSI(), func() bool {
		_, ok := m.LookupUeBySupi(supi)
		return !ok
	})
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

func (m *MME) ReconcileRegistrations(ctx context.Context) {
	if m.Registrations == nil {
		return
	}

	for _, ue := range m.registeredUEs() {
		imsi := ue.imsiOrEmpty()
		if imsi == "" || !m.Registrations.Superseded(ctx, imsi, ue.registeredAt.Load()) {
			continue
		}

		m.releaseSuperseded(ctx, ue)
	}
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

	if ue.IdleMobilityTo5GSPending() {
		return
	}

	m.ReleaseAllSessions(ctx, ue)
	ue.TransitionTo(ctx, EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)

	logger.From(ctx, logger.MmeLog).Info("UE registered on another node; dropping its local EPS registration and PDN connections",
		logger.SUPI(supi.String()))
}
