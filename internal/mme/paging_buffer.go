// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "sync"

type pagingBuffer[T any] struct {
	mu sync.Mutex
	v  *T
}

func (b *pagingBuffer[T]) set(v T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.v = &v
}

func (b *pagingBuffer[T]) pop() *T {
	b.mu.Lock()
	defer b.mu.Unlock()

	v := b.v
	b.v = nil

	return v
}

func (b *pagingBuffer[T]) clear() {
	b.clearIf(func(*T) bool { return true })
}

func (b *pagingBuffer[T]) clearIf(match func(*T) bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.v != nil && match(b.v) {
		b.v = nil
	}
}

func (b *pagingBuffer[T]) pending() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.v != nil
}

func (ue *UeContext) clearPagingBuffers() {
	ue.lppaBuf.clear()
	ue.lppBuf.clear()
}
