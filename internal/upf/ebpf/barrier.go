// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"errors"
	"fmt"

	"github.com/cilium/ebpf"
)

type ProgramBarrier struct {
	outer *ebpf.Map
	inner *ebpf.Map
}

func NewProgramBarrier() (*ProgramBarrier, error) {
	innerSpec := &ebpf.MapSpec{
		Type:       ebpf.Array,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	}

	inner, err := ebpf.NewMap(innerSpec)
	if err != nil {
		return nil, fmt.Errorf("create barrier inner map: %w", err)
	}

	outer, err := ebpf.NewMap(&ebpf.MapSpec{
		Type:       ebpf.ArrayOfMaps,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		InnerMap:   innerSpec,
	})
	if err != nil {
		_ = inner.Close()

		return nil, fmt.Errorf("create barrier outer map: %w", err)
	}

	return &ProgramBarrier{outer: outer, inner: inner}, nil
}

func (p *ProgramBarrier) Wait() error {
	return p.outer.Put(uint32(0), p.inner)
}

func (p *ProgramBarrier) Close() error {
	return errors.Join(p.outer.Close(), p.inner.Close())
}
