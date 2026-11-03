// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/ueregistration"
	"github.com/ellanetworks/core/nas/eps"
)

type Registrar interface {
	Register(ctx context.Context, imsi string) (int64, error)
	RegisterVisitor(ctx context.Context, imsi, partnerID string) (int64, error)
	Confirmed(ctx context.Context, imsi string, version int64) bool
	Purge(imsi string)
	Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context), withdraw func(context.Context, ueregistration.Withdrawal))
}

func (m *MME) RegisterUE(ctx context.Context, ue *UeContext) error {
	if m.Registrations == nil {
		return nil
	}

	var (
		version int64
		err     error
	)

	if v, ok := ue.Visitor(); ok {
		version, err = m.Registrations.RegisterVisitor(ctx, ue.IMSI(), v.PartnerID)
	} else {
		version, err = m.Registrations.Register(ctx, ue.IMSI())
	}

	if errors.Is(err, db.ErrRoamingRefused) {
		return &Rejection{Cause: eps.EMMCausePLMNNotAllowed, Reason: "the roaming agreement is being withdrawn"}
	}

	if err != nil {
		return err
	}

	ue.registrationVersion.Store(version)

	return nil
}

func (m *MME) ConfirmRegistration(ctx context.Context, ue *UeContext) error {
	m.ConfirmHomeLocation(ctx, ue)

	if m.Registrations == nil {
		return nil
	}

	if m.Registrations.Confirmed(ctx, ue.IMSI(), ue.registrationVersion.Load()) {
		return nil
	}

	return m.RegisterUE(ctx, ue)
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
	}, func(ctx context.Context, why ueregistration.Withdrawal) {
		if why == ueregistration.SubscriberDeleted {
			m.withdrawSubscription(ctx, imsi)
			return
		}

		m.withdrawRoamer(ctx, ue, why)
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

	ue.setHomeRegistered(false)

	m.ReleaseAllSessions(ctx, ue, models.ReleaseFor(models.ReleaseDeregistration))
	ue.TransitionTo(ctx, EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, CauseNASNormalRelease)

	logger.From(ctx, logger.MmeLog).Info("UE registered on another node; dropping its local EPS registration and PDN connections",
		logger.SUPI(supi.String()))
}

func (m *MME) withdrawSubscription(ctx context.Context, imsi string) {
	logger.From(ctx, logger.MmeLog).Info("subscriber deleted; detaching UE", logger.SUPIFromIMSI(imsi))

	m.DetachSubscriber(ctx, imsi)
	m.ForgetSubscriber(imsi)
}
