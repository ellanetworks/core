// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

const activateDefaultBearerName = "Activate Default EPS Bearer Context Request"

type pdnActivation struct {
	pti     uint8
	request []byte
	plain   []byte
}

func (m *MME) BeginPDNActivation(ctx context.Context, ue *UeContext, p *PdnConnection, pti uint8, request []byte, plain []byte) {
	ue.mu.Lock()
	p.ESM = ESMActivePending
	p.activation = &pdnActivation{pti: pti, request: request, plain: plain}
	ue.mu.Unlock()

	m.armPDNActivationGuard(ctx, ue, p, plain)
}

func (m *MME) armPDNActivationGuard(ctx context.Context, ue *UeContext, p *PdnConnection, plain []byte) {
	m.ArmESMGuardAbortOnly(ctx, ue, p, activateDefaultBearerName, plain, eps.SHTIntegrityProtectedCiphered, func(ctx context.Context) {
		m.ReleasePDN(ctx, ue, p, models.ReleaseFor(models.ReleaseEstablishmentFailure))
	})
}

func (m *MME) CompletePDNActivation(ue *UeContext, p *PdnConnection) {
	m.StopESMGuard(p)

	ue.mu.Lock()
	defer ue.mu.Unlock()

	p.ESM = ESMActive
	p.activation = nil
}

func (m *MME) ResendPDNActivation(ctx context.Context, ue *UeContext, p *PdnConnection, request []byte) bool {
	ue.mu.Lock()
	act := p.activation
	identical := ue.Pdns[p.Ebi] == p && p.ESM == ESMActivePending && act != nil && len(request) > 0 && bytes.Equal(act.request, request)
	ue.mu.Unlock()

	if !identical {
		return false
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return false
	}

	m.armPDNActivationGuard(ctx, ue, p, act.plain)

	if err := ueConn.SendProtectedNASTransport(ctx, act.plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		ReportProtectFailure(ctx, ueConn, activateDefaultBearerName, err)
	}

	return true
}

func (m *MME) RejectPDNActivation(ctx context.Context, ue *UeContext, p *PdnConnection, reason string, release models.SessionReleaseRequest) bool {
	ue.mu.Lock()
	act := p.activation
	pending := ue.Pdns[p.Ebi] == p && p.ESM == ESMActivePending && act != nil

	if pending {
		p.activation = nil
	}
	ue.mu.Unlock()

	if !pending {
		return false
	}

	m.StopESMGuard(p)

	cause := eps.ESMCauseServiceOptionTemporarilyOut

	logger.From(ctx, logger.MmeLog).Warn("PDN connectivity rejected: "+reason,
		logger.SUPI(ue.Supi().String()), zap.String("apn", p.Apn), logger.ERABID(p.Ebi),
		zap.Uint8("pti", act.pti), logger.ESMCause(cause.String()))

	ue.Conn().SendDownlinkProtected(ctx, &eps.PDNConnectivityReject{
		PTI:   nas.ProcedureTransactionIdentity(act.pti),
		Cause: cause,
	})

	m.ReleasePDN(ctx, ue, p, release.WithSentCause(uint8(cause)))

	return true
}
