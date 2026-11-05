// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"go.uber.org/zap"
)

func ForEachConcurrently[T any](ctx context.Context, items []T, concurrency int, verb string, fn func(T) error) error {
	var (
		wg       sync.WaitGroup
		done     atomic.Int64
		failed   atomic.Int64
		firstErr atomic.Pointer[error]
		sem      = make(chan struct{}, concurrency)
		stop     = make(chan struct{})
	)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				logger.Logger.Info("progress", zap.String("phase", verb), zap.Int64("done", done.Load()), zap.Int("total", len(items)), zap.Int64("failed", failed.Load()))
			}
		}
	}()

	defer close(stop)

	for _, item := range items {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			if err := fn(item); err != nil {
				failed.Add(1)
				firstErr.CompareAndSwap(nil, &err)

				return
			}

			done.Add(1)
		})
	}

	wg.Wait()

	if err := firstErr.Load(); err != nil {
		return fmt.Errorf("%d of %d UEs not %s, first: %w", failed.Load(), len(items), verb, *err)
	}

	return ctx.Err()
}

func LatencyFields(latencies []time.Duration) []zap.Field {
	sorted := slices.Sorted(slices.Values(latencies))

	pct := func(q float64) time.Duration { return sorted[min(int(float64(len(sorted))*q), len(sorted)-1)] }

	return []zap.Field{zap.Duration("p50", pct(0.5)), zap.Duration("p95", pct(0.95)), zap.Duration("p99", pct(0.99)), zap.Duration("max", pct(1))}
}
