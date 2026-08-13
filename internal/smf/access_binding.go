// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type bindingRules struct {
	policyID string
	pdrs     []*PDR
	fars     []*FAR
	qers     []*QER
}

type accessBinding struct {
	access         AccessType
	keepOnRollback bool
	build          func(commit *transferCommit) (bindingRules, error)
	onBound        func()
}

func (s *SMF) commitAccessBinding(ctx context.Context, sc *SMContext, b accessBinding) (*droppedSource, error) {
	span := trace.SpanFromContext(ctx)

	commit, err := s.beginTransferCommit(ctx, sc, b.access)
	if err != nil {
		return nil, failBinding(span, "failed to move the session", fmt.Errorf("failed to move session %q to %s: %w", sc.Ref, b.access, err))
	}

	restoreBinding := sc.stageAccessBinding()

	rollback := func(err error) error {
		restoreBinding()

		if commit == nil {
			return failBinding(span, "failed to bind the downlink", err)
		}

		commit.restore()

		if b.keepOnRollback {
			return failBinding(span, "failed to bind the downlink", err)
		}

		sc.releasing = true

		return failBinding(span, "failed to bind the downlink", fmt.Errorf("%w: %v", errTransferRolledBack, err))
	}

	rules, err := b.build(commit)
	if err != nil {
		return nil, rollback(err)
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		sc.PFCPContext.SEID,
		rules.policyID,
		rules.pdrs,
		rules.fars,
		rules.qers,
	)); err != nil {
		return nil, rollback(fmt.Errorf("failed to send PFCP session modification request: %w", err))
	}

	var dropped *droppedSource
	if commit != nil {
		dropped = sc.finishTransferCommit(commit)
	}

	if b.onBound != nil {
		b.onBound()
	}

	return dropped, nil
}

func failBinding(span trace.Span, status string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, status)

	return err
}

func (sc *SMContext) pendingTransferTo(access AccessType) bool {
	return sc.pending != nil && sc.pending.to == access
}

func (s *SMF) finishAccessBinding(ctx context.Context, sc *SMContext, dropped *droppedSource, err error) error {
	if err != nil {
		if errors.Is(err, errTransferRolledBack) {
			s.reportSessionNotMovedTo5GS(ctx, sc)

			if releaseErr := s.releaseSession(ctx, sc.Ref); releaseErr != nil {
				logger.WithTrace(ctx, logger.SmfLog).Warn("failed to release a session whose move was rolled back",
					zap.Error(releaseErr), zap.String("ref", sc.Ref))
			}
		}

		return err
	}

	s.dropSourceRouting(ctx, sc.Ref, dropped)

	return nil
}
