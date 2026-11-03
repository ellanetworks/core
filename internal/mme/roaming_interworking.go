// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

type ULRKind int

const (
	ULRInitialAttach ULRKind = iota
	ULRRestoration
	ULRInterSystem
	ULRInterSystemHandover
)

func (k ULRKind) flags() s6a.ULRFlags {
	flags := s6a.ULRFlagS6aS6dIndicator

	switch k {
	case ULRInitialAttach:
		flags |= s6a.ULRFlagInitialAttachIndicator
	case ULRInterSystemHandover:
		flags |= s6a.ULRFlagIntraPLMNInterMMEHandover
	}

	return flags
}

func (m *MME) admitArrivingVisitor(ctx context.Context, ue *UeContext) error {
	err := m.AdmitArrivingVisitor(ctx, ue)

	var r *Rejection
	if errors.As(err, &r) {
		return fmt.Errorf("%w: %w", r, interworking.TargetRefusal{Cause: s1ap.Cause{
			Group: s1ap.CauseGroupRadioNetwork,
			Value: s1ap.CauseRadioNetworkHOTargetNotAllowed,
		}})
	}

	return err
}

func (m *MME) AdmitArrivingVisitor(ctx context.Context, ue *UeContext) error {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return fmt.Errorf("mme: resolve the serving PLMN: %w", err)
	}

	return m.AdmitVisitor(ctx, ue, plmn)
}

func (m *MME) HomeRegistrationDue(ue *UeContext) bool {
	v, ok := ue.Visitor()

	return ok && !v.Registered
}

func (m *MME) RequestInterSystemUpdateLocation(ctx context.Context, ue *UeContext, conn *UeConn, serving models.PlmnID, done func(context.Context, error)) {
	kind := ULRInterSystem
	if ue.arrivedByHandoverFrom5GS.Load() {
		kind = ULRInterSystemHandover
	}

	var err error

	m.awaitHome(ctx, ue, conn, "Update-Location", func(ctx context.Context) {
		err = m.updateLocation(ctx, ue, serving, kind)
	}, func(ctx context.Context) {
		if err == nil {
			ue.arrivedByHandoverFrom5GS.Store(false)
		}

		done(ctx, err)
	})
}

func (m *MME) handVisitorTo5GS(ctx context.Context, ue *UeContext) {
	v, ok := ue.Visitor()
	if !ok {
		return
	}

	ue.setHomeRegistered(false)

	logger.From(ctx, logger.MmeLog).Info("visitor moved to 5GS; its home registration passes to the AMF, no purge",
		logger.SUPI(ue.Supi().String()), zap.String("roaming_partner", v.PartnerName))
}

func (m *MME) movingTo5GS(ue *UeContext) bool {
	if ue.IdleMobilityTo5GSPending() {
		return true
	}

	_, relocating := m.RelocationToFiveGS(ue)

	return relocating
}
