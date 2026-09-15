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
}

type pagingProc struct {
	mu       sync.Mutex
	state    PagingState
	pending  *MTRequest
	deferred *MTRequest
	guard    guard.Guard
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

func (ue *UeContext) deferServiceRequest(req *MTRequest) {
	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	ue.paging.deferred = req
}

func (ue *UeContext) takeDeferredServiceRequest() *MTRequest {
	if ue == nil {
		return nil
	}

	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	req := ue.paging.deferred
	ue.paging.deferred = nil

	return req
}

func (ue *UeContext) beginPaging(req *MTRequest) {
	ue.paging.mu.Lock()
	defer ue.paging.mu.Unlock()

	ue.paging.pending = req
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

func (ue *UeContext) PagingDelivered(ctx context.Context) {
	if ue == nil {
		return
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()

	ue.paging.pending = nil
	ue.paging.state = PagingIdle

	ue.paging.mu.Unlock()

	ue.Conn().ResumeDeferredReleaseIfSettled(ctx)
}

func (ue *UeContext) PagingFailed(ctx context.Context, cause models.EPSPagingFailureCause) *MTRequest {
	if ue == nil {
		return nil
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()
	dropped := ue.takePendingLocked()
	ue.paging.mu.Unlock()

	ue.ClearLPPaBuffered()

	if dropped != nil {
		ue.notifyMTDeliveryFailure(ctx, dropped, cause)
	}

	ue.Conn().ResumeDeferredReleaseIfSettled(ctx)

	return dropped
}

func (ue *UeContext) PagingUnanswered(ctx context.Context, cause models.EPSPagingFailureCause) (*MTRequest, bool) {
	if ue == nil {
		return nil, false
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()

	if ue.Connected() {
		ue.paging.mu.Unlock()

		return nil, false
	}

	dropped := ue.takePendingLocked()

	ue.paging.mu.Unlock()

	ue.ClearLPPaBuffered()

	if dropped != nil {
		ue.notifyMTDeliveryFailure(ctx, dropped, cause)
	}

	return dropped, true
}

func (ue *UeContext) settleDeliveryOnRelease(ctx context.Context) {
	if ue.PagingState() == PagingDelivering {
		ue.PagingFailed(ctx, models.EPSPagingUENotResponding)
	}
}

func (ue *UeContext) takePendingLocked() *MTRequest {
	dropped := ue.paging.pending
	ue.paging.pending = nil
	ue.paging.state = PagingIdle

	return dropped
}

func (ue *UeContext) notifyMTDeliveryFailure(ctx context.Context, req *MTRequest, cause models.EPSPagingFailureCause) {
	if ue.session == nil || req == nil || req.Ebi == 0 {
		return
	}

	imsi := ue.imsiOrEmpty()

	if err := ue.session.HandleEPSPagingFailure(ctx, imsi, req.Ebi, cause); err != nil {
		logger.MmeLog.Warn("could not report an EPS downlink data notification failure",
			logger.SUPIFromIMSI(imsi),
			zap.Uint8("ebi", req.Ebi),
			logger.Cause(cause.String()),
			zap.Error(err))
	}
}
