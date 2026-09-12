// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

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
	Req models.N1N2MessageTransferRequest
}

func (r *MTRequest) Request() *models.N1N2MessageTransferRequest {
	if r == nil {
		return nil
	}

	return &r.Req
}

func outranks(candidate, current *models.Arp) bool {
	if candidate == nil {
		return false
	}

	if current == nil {
		return false
	}

	return candidate.PriorityLevel < current.PriorityLevel
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

func (ue *UeContext) beginPaging(req *MTRequest) (models.N1N2MessageTransferCause, error) {
	ue.paging.mu.Lock()

	if ue.paging.state == PagingAttempting && !outranks(req.arp(), ue.paging.pending.arp()) {
		rejected := &models.N1N2MessageTransferError{
			Cause:  models.N1N2ErrHigherPriorityRequestOngoing,
			Detail: models.N1N2MsgTxfrErrDetail{HighestPrioArp: ue.paging.pending.arp()},
		}

		ue.paging.mu.Unlock()

		return "", rejected
	}

	displaced := ue.paging.pending
	ue.paging.pending = req
	ue.paging.state = PagingAttempting

	ue.paging.mu.Unlock()

	if displaced != nil && !sameDelivery(displaced, req) {
		ue.notifyMTDeliveryFailure(displaced, models.N1N2FailureCauseUnspecified)
	}

	return models.N1N2AttemptingToReachUE, nil
}

func sameDelivery(a, b *MTRequest) bool {
	if a.Req.Standalone() || b.Req.Standalone() {
		return false
	}

	return a.Req.PduSessionID == b.Req.PduSessionID
}

func (r *MTRequest) arp() *models.Arp {
	if r == nil {
		return nil
	}

	return r.Req.Arp
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

func (ue *UeContext) PagingFailed(cause models.N1N2MessageTransferCause) *MTRequest {
	if ue == nil {
		return nil
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()
	dropped := ue.takePendingLocked()
	ue.paging.mu.Unlock()

	if dropped != nil {
		ue.notifyMTDeliveryFailure(dropped, cause)
	}

	ue.Conn().ResumeDeferredReleaseIfSettled()

	return dropped
}

func (ue *UeContext) PagingUnanswered(cause models.N1N2MessageTransferCause) (*MTRequest, bool) {
	if ue == nil {
		return nil, false
	}

	ue.paging.guard.Stop()

	ue.paging.mu.Lock()

	if ue.Conn() != nil {
		ue.paging.mu.Unlock()

		return nil, false
	}

	dropped := ue.takePendingLocked()

	ue.paging.mu.Unlock()

	if dropped != nil {
		ue.notifyMTDeliveryFailure(dropped, cause)
	}

	return dropped, true
}

func (ue *UeContext) PagingAttemptFailed(req *MTRequest, cause models.N1N2MessageTransferCause) {
	if ue == nil {
		return
	}

	ue.paging.mu.Lock()

	if ue.paging.pending != req {
		ue.paging.mu.Unlock()

		return
	}

	dropped := ue.takePendingLocked()

	ue.paging.mu.Unlock()

	ue.paging.guard.Stop()

	if dropped != nil {
		ue.notifyMTDeliveryFailure(dropped, cause)
	}

	ue.Conn().ResumeDeferredReleaseIfSettled()
}

func (ue *UeContext) takePendingLocked() *MTRequest {
	dropped := ue.paging.pending
	ue.paging.pending = nil
	ue.paging.state = PagingIdle

	return dropped
}

func (ue *UeContext) notifyMTDeliveryFailure(req *MTRequest, cause models.N1N2MessageTransferCause) {
	if ue.smf == nil || req == nil || req.Req.Standalone() {
		return
	}

	if err := ue.smf.HandleN1N2TransferFailure(context.Background(), ue.Supi(), req.Req.PduSessionID, cause); err != nil {
		logger.AmfLog.Warn("could not report an N1N2 transfer failure",
			logger.SUPI(ue.Supi().String()),
			zap.Uint8("pdu_session_id", req.Req.PduSessionID),
			zap.String("cause", cause.String()),
			zap.Error(err))
	}
}
