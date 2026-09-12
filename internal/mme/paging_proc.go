// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

type PagingState uint8

const (
	PagingIdle PagingState = iota
	PagingAttempting
	PagingDelivering
)

func (s PagingState) String() string {
	switch s {
	case PagingIdle:
		return "Idle"
	case PagingAttempting:
		return "Attempting"
	case PagingDelivering:
		return "Delivering"
	default:
		return "Unknown"
	}
}

type MTRequest struct {
	Ebi uint8
	Arp *models.Arp
}

type pagingProc struct {
	mu      sync.Mutex
	state   PagingState
	pending *MTRequest
	guard   guard.Guard
}

func (ue *UeContext) PagingState() PagingState {
	if ue == nil {
		return PagingIdle
	}

	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	return ue.paging.state
}

func (ue *UeContext) PagingPending() *MTRequest {
	if ue == nil {
		return nil
	}

	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	return ue.paging.pending
}

func (ue *UeContext) MTDeliveryInProgress() bool {
	return ue.PagingState() != PagingIdle
}

func (ue *UeContext) beginPaging(req *MTRequest) {
	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	if req != nil {
		ue.paging.pending = req
	}

	ue.paging.state = PagingAttempting
}

func (ue *UeContext) PagingAnswered() {
	if ue == nil {
		return
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	if ue.paging.state == PagingAttempting {
		ue.paging.state = PagingDelivering
	}
}

func (ue *UeContext) PagingDelivered() {
	if ue == nil {
		return
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()

	ue.paging.pending = nil
	ue.paging.state = PagingIdle

	ue.paging.mu.Unlock()

	ue.Conn().ResumeDeferredReleaseIfSettled()
}

func (m *MME) PagingFailed(ue *UeContext, cause models.EPSPagingFailureCause) *MTRequest {
	if ue == nil {
		return nil
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()

	dropped := ue.paging.pending
	ue.paging.pending = nil
	ue.paging.state = PagingIdle

	ue.paging.mu.Unlock()

	ue.ClearLPPaBuffered()

	if dropped != nil {
		m.notifyEPSPagingFailure(ue, dropped.Ebi, cause)
	}

	ue.Conn().ResumeDeferredReleaseIfSettled()

	return dropped
}

func (m *MME) notifyEPSPagingFailure(ue *UeContext, ebi uint8, cause models.EPSPagingFailureCause) {
	if m.Session == nil {
		return
	}

	imsi := ue.imsiOrEmpty()

	if err := m.Session.HandleEPSPagingFailure(context.Background(), imsi, ebi, cause); err != nil {
		logger.MmeLog.Warn("could not report an EPS downlink data notification failure",
			zap.String("imsi", imsi), zap.Uint8("ebi", ebi), zap.String("cause", cause.String()), zap.Error(err))
	}
}

func (ue *UeContext) PagingActive() bool {
	return ue.PagingState() == PagingAttempting
}
