// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cilium/ebpf"
)

type ProgramWaiter interface {
	Wait() error
}

type ProgramBarrier struct {
	outer *ebpf.Map
	inner *ebpf.Map

	mu        sync.Mutex
	done      *sync.Cond
	started   uint64
	completed uint64
	running   bool
	err       error
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

	p := &ProgramBarrier{outer: outer, inner: inner}
	p.done = sync.NewCond(&p.mu)

	return p, nil
}

func (p *ProgramBarrier) Wait() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	target := p.started + 1

	for p.completed < target {
		if p.running {
			p.done.Wait()
			continue
		}

		p.running = true
		p.started++
		generation := p.started

		p.mu.Unlock()
		err := p.outer.Put(uint32(0), p.inner)
		p.mu.Lock()

		p.running = false
		p.completed = generation
		p.err = err
		p.done.Broadcast()
	}

	return p.err
}

func (p *ProgramBarrier) Close() error {
	return errors.Join(p.outer.Close(), p.inner.Close())
}
