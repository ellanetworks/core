// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enginetest

import (
	"net/netip"
	"sync"

	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

type Op string

const (
	OpPutPdrUplink         Op = "PutPdrUplink"
	OpPutPdrDownlink       Op = "PutPdrDownlink"
	OpDeletePdrUplink      Op = "DeletePdrUplink"
	OpDeletePdrDownlink    Op = "DeletePdrDownlink"
	OpPutFramedDownlink    Op = "PutFramedDownlink"
	OpDeleteFramedDownlink Op = "DeleteFramedDownlink"
	OpNewUrr               Op = "NewUrr"
	OpDeleteUrr            Op = "DeleteUrr"
	OpPutSdfFilterList     Op = "PutSdfFilterList"
	OpDeleteSdfFilterList  Op = "DeleteSdfFilterList"
)

type FaultyMaps struct {
	*Maps

	mu     sync.Mutex
	faults map[Op]error
}

func NewFaultyMaps(m *Maps) *FaultyMaps {
	return &FaultyMaps{Maps: m, faults: map[Op]error{}}
}

func (f *FaultyMaps) Fail(op Op, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.faults[op] = err
}

func (f *FaultyMaps) Heal(op Op) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.faults, op)
}

func (f *FaultyMaps) fault(op Op) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.faults[op]
}

func (f *FaultyMaps) PutPdrUplink(teid uint32, pdrInfo upfebpf.PdrInfo) error {
	if err := f.fault(OpPutPdrUplink); err != nil {
		return err
	}

	return f.Maps.PutPdrUplink(teid, pdrInfo)
}

func (f *FaultyMaps) PutPdrDownlink(addr netip.Addr, pdrInfo upfebpf.PdrInfo) error {
	if err := f.fault(OpPutPdrDownlink); err != nil {
		return err
	}

	return f.Maps.PutPdrDownlink(addr, pdrInfo)
}

func (f *FaultyMaps) DeletePdrUplink(teid uint32) error {
	if err := f.fault(OpDeletePdrUplink); err != nil {
		return err
	}

	return f.Maps.DeletePdrUplink(teid)
}

func (f *FaultyMaps) DeletePdrDownlink(addr netip.Addr) error {
	if err := f.fault(OpDeletePdrDownlink); err != nil {
		return err
	}

	return f.Maps.DeletePdrDownlink(addr)
}

func (f *FaultyMaps) PutFramedDownlink(prefix netip.Prefix, ueAddr netip.Addr) error {
	if err := f.fault(OpPutFramedDownlink); err != nil {
		return err
	}

	return f.Maps.PutFramedDownlink(prefix, ueAddr)
}

func (f *FaultyMaps) DeleteFramedDownlink(prefix netip.Prefix) error {
	if err := f.fault(OpDeleteFramedDownlink); err != nil {
		return err
	}

	return f.Maps.DeleteFramedDownlink(prefix)
}

func (f *FaultyMaps) NewUrr(seid uint64, id uint32) error {
	if err := f.fault(OpNewUrr); err != nil {
		return err
	}

	return f.Maps.NewUrr(seid, id)
}

func (f *FaultyMaps) DeleteUrr(seid uint64, id uint32) error {
	if err := f.fault(OpDeleteUrr); err != nil {
		return err
	}

	return f.Maps.DeleteUrr(seid, id)
}

func (f *FaultyMaps) PutSdfFilterList(index uint32, list upfebpf.SdfFilterList) error {
	if err := f.fault(OpPutSdfFilterList); err != nil {
		return err
	}

	return f.Maps.PutSdfFilterList(index, list)
}

func (f *FaultyMaps) DeleteSdfFilterList(index uint32) error {
	if err := f.fault(OpDeleteSdfFilterList); err != nil {
		return err
	}

	return f.Maps.DeleteSdfFilterList(index)
}
