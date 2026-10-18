// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"

	bpf "github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

type ringPositions interface {
	producer() uintptr
	consumer() uintptr
	Close() error
}

type ringCursor struct {
	consPage []byte
	prodPage []byte
}

func openRingCursor(m *bpf.Map) (*ringCursor, error) {
	pageSize := os.Getpagesize()

	cons, err := unix.Mmap(m.FD(), 0, pageSize, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap consumer page: %w", err)
	}

	prod, err := unix.Mmap(m.FD(), int64(pageSize), pageSize, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		_ = unix.Munmap(cons)

		return nil, fmt.Errorf("mmap producer page: %w", err)
	}

	return &ringCursor{consPage: cons, prodPage: prod}, nil
}

func (c *ringCursor) producer() uintptr {
	return atomic.LoadUintptr((*uintptr)(unsafe.Pointer(&c.prodPage[0])))
}

func (c *ringCursor) consumer() uintptr {
	return atomic.LoadUintptr((*uintptr)(unsafe.Pointer(&c.consPage[0])))
}

func (c *ringCursor) Close() error {
	return errors.Join(unix.Munmap(c.prodPage), unix.Munmap(c.consPage))
}

func positionBefore(pos, target uintptr) bool {
	return int(pos-target) < 0
}
