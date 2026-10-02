// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
)

type afterAnswerKey struct{}

type afterAnswer struct {
	mu    sync.Mutex
	done  bool
	funcs []func(error)
}

func AfterAnswer(ctx context.Context, fn func(err error)) bool {
	a, ok := ctx.Value(afterAnswerKey{}).(*afterAnswer)
	if !ok {
		return false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return false
	}

	a.funcs = append(a.funcs, fn)

	return true
}

func (a *afterAnswer) run(c *Conn, err error) {
	a.mu.Lock()
	a.done = true
	funcs := a.funcs
	a.funcs = nil
	a.mu.Unlock()

	for _, fn := range funcs {
		c.runAfterAnswer(fn, err)
	}
}

func (c *Conn) runAfterAnswer(fn func(error), err error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger().Error("panic in Diameter after-answer function",
				slog.Any("panic", r), slog.String("stack", string(debug.Stack())))
		}
	}()

	fn(err)
}
