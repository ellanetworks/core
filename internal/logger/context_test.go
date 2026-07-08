// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func TestFromReturnsBaseWhenNoLoggerStored(t *testing.T) {
	base := zap.NewNop()

	got := logger.From(context.Background(), base)
	if got != base {
		t.Errorf("expected base logger, got a different logger")
	}
}

func TestFromReturnsStoredLogger(t *testing.T) {
	base := zap.NewNop()
	stored := zap.NewNop()

	ctx := logger.Into(context.Background(), stored)

	got := logger.From(ctx, base)
	if got != stored {
		t.Errorf("expected stored logger, got base or other logger")
	}
}

func TestFromReturnsBaseWhenStoredLoggerNil(t *testing.T) {
	base := zap.NewNop()

	ctx := logger.Into(context.Background(), nil)

	got := logger.From(ctx, base)
	if got != base {
		t.Errorf("expected base logger for a nil stored logger, got a different logger")
	}
}
