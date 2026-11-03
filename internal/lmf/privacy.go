// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

var ErrPrivacyRefused = errors.New("location refused by the roamer's home privacy profile")

type PrivacySource interface {
	LocationPrivacy(ctx context.Context, supi etsi.SUPI) error
}

func (l *LMF) privacySources() []PrivacySource {
	srcs := make([]PrivacySource, 0, 2)

	if l.amf != nil {
		srcs = append(srcs, l.amf)
	}

	if l.mme != nil {
		srcs = append(srcs, l.mme)
	}

	return srcs
}

func (l *LMF) checkPrivacy(ctx context.Context, supi etsi.SUPI) error {
	err := l.privacyRefusal(ctx, supi)
	if err == nil {
		return nil
	}

	logger.LmfLog.Info("location refused by the roamer's home privacy profile", logger.SUPI(supi.String()), zap.Error(err))

	return fmt.Errorf("%w: %w", ErrPrivacyRefused, err)
}

func (l *LMF) privacyRefusal(ctx context.Context, supi etsi.SUPI) error {
	if l.privacyOverride != nil {
		return l.privacyOverride(ctx, supi)
	}

	for _, s := range l.privacySources() {
		if err := s.LocationPrivacy(ctx, supi); err != nil {
			return err
		}
	}

	return nil
}
