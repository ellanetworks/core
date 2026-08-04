// Copyright 2023 Edgecom LLC
// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Modified by Ella Networks.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ebpf

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// BPF_CFLAGS selects optional instrumentation: -DENABLE_LOG for debug output
// on the trace pipe, -DENABLE_PROFILING for per-packet latency in
// profiling_map, read back by ReadProfilingStats. Profiling costs ~600-900 ns
// per sample.

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3N6Entrypoint bpf/n3n6_bpf.c -- -I. -O2 -Wall -Werror -g
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3N6EntrypointTc bpf/n3n6_bpf.c -- -DCTX_TC -I. -O2 -Wall -Werror -g

// Masquerade source-port range, held below the kernel's default ephemeral
// floor so an allocation cannot name a port a host socket is using.
const (
	NatPortMin uint16 = 1024
	NatPortMax uint16 = 32767
)

type DataNotification struct {
	LocalSEID uint64
	PdrID     uint16
	QFI       uint8
}

// RSEvent is the Go representation of struct rs_event emitted by the N3 XDP
// program when a Router Solicitation is detected inside a GTP-U packet.
type RSEvent struct {
	TEID   uint32
	UEIPv6 [16]byte // struct in6_addr — UE source IPv6 address
}

type BpfObjects struct {
	N3N6EntrypointObjects

	// ProfilingMap is the optional per-CPU profiling map. It is non-nil only
	// when the BPF code was compiled with -DENABLE_PROFILING. The field
	// shadows the same-named field promoted from N3N6EntrypointMaps so that
	// package code can reference bpfObjects.ProfilingMap unconditionally
	// without a compile error when profiling is absent.
	ProfilingMap *ebpf.Map

	FlowAccounting bool
	Masquerade     bool
	// UseTCX selects the SCHED_CLS build of the datapath. Its spec carries
	// the same map, program, and variable names as the XDP build (one C
	// source), so it loads into N3N6EntrypointObjects through the ebpf
	// struct tags.
	UseTCX           bool
	N3InterfaceIndex uint32
	N6InterfaceIndex uint32
	N3Vlan           uint32
	N6Vlan           uint32
	pagingMu         sync.Mutex
	pagingList       map[DataNotification]bool
}

func NewBpfObjects(flowact bool, masquerade bool, n3ifindex int, n6ifindex int, n3vlan uint32, n6vlan uint32) *BpfObjects {
	return &BpfObjects{
		FlowAccounting:   flowact,
		Masquerade:       masquerade,
		N3InterfaceIndex: uint32(n3ifindex),
		N6InterfaceIndex: uint32(n6ifindex),
		N3Vlan:           n3vlan,
		N6Vlan:           n6vlan,
		pagingList:       make(map[DataNotification]bool),
	}
}

// Tail-call indices in the upf_calls PROG_ARRAY; must match UPF_CALL_* in
// bpf/n3n6_bpf.c.
const (
	upfCallUplink      = 0
	upfCallDownlink    = 1
	upfCallGtpuControl = 2
)

// populateTailCalls inserts the stage programs into the upf_calls PROG_ARRAY so
// upf_entry_func can tail-call them. Must run after every load/reload: the array
// holds program FDs, which are new on each load.
func populateTailCalls(objs *N3N6EntrypointObjects) error {
	if err := objs.UpfCalls.Put(uint32(upfCallUplink), objs.UpfUplinkFunc); err != nil {
		return fmt.Errorf("put uplink stage in upf_calls: %w", err)
	}

	if err := objs.UpfCalls.Put(uint32(upfCallDownlink), objs.UpfDownlinkFunc); err != nil {
		return fmt.Errorf("put downlink stage in upf_calls: %w", err)
	}

	if err := objs.UpfCalls.Put(uint32(upfCallGtpuControl), objs.UpfGtpuControlFunc); err != nil {
		return fmt.Errorf("put gtpu control stage in upf_calls: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) loadSpec() (*ebpf.CollectionSpec, error) {
	if bpfObjects.UseTCX {
		return LoadN3N6EntrypointTc()
	}

	return LoadN3N6Entrypoint()
}

func (bpfObjects *BpfObjects) Load() error {
	n3n6Spec, err := bpfObjects.loadSpec()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}

	// The csum_scratch map is a regular BPF_MAP_TYPE_ARRAY indexed by CPU
	// ID (not a per-CPU map, because the per-CPU allocator rejects values
	// larger than ~32 KB).  Set max_entries to the number of CPUs so every
	// CPU gets its own slot.
	if m, ok := n3n6Spec.Maps["csum_scratch"]; ok {
		m.MaxEntries = uint32(runtime.NumCPU())
	}

	if err := bpfObjects.loadAndAssignFromSpec(n3n6Spec, &bpfObjects.N3N6EntrypointObjects, nil); err != nil {
		logger.UpfLog.Error("failed to load N3/N6 program", zap.Error(err))
		return err
	}

	if err := populateTailCalls(&bpfObjects.N3N6EntrypointObjects); err != nil {
		logger.UpfLog.Error("failed to populate tail-call program array", zap.Error(err))
		return err
	}

	// Populate the optional profiling map from the embedded maps struct. The
	// field only exists in N3N6EntrypointMaps when compiled with
	// -DENABLE_PROFILING; reflection lets us check without a hard reference.
	bpfObjects.ProfilingMap = profilingMapFromMaps(bpfObjects.N3N6EntrypointMaps)

	return nil
}

func (bpfObjects *BpfObjects) loadAndAssignFromSpec(spec *ebpf.CollectionSpec, to any, opts *ebpf.CollectionOptions) error {
	if err := spec.Variables["flowact"].Set(bpfObjects.FlowAccounting); err != nil {
		logger.UpfLog.Error("failed to set flow accounting value", zap.Error(err))
		return err
	}

	if err := spec.Variables["masquerade"].Set(bpfObjects.Masquerade); err != nil {
		logger.UpfLog.Error("failed to set masquerade value", zap.Error(err))
		return err
	}

	if err := spec.Variables["n3_ifindex"].Set(bpfObjects.N3InterfaceIndex); err != nil {
		logger.UpfLog.Error("failed to set n3 interface index", zap.Error(err))
		return err
	}

	if err := spec.Variables["n6_ifindex"].Set(bpfObjects.N6InterfaceIndex); err != nil {
		logger.UpfLog.Error("failed to set n6 interface index", zap.Error(err))
		return err
	}

	if err := spec.Variables["n3_vlan"].Set(bpfObjects.N3Vlan); err != nil {
		logger.UpfLog.Error("failed to set n3 vlan id", zap.Error(err))
		return err
	}

	if err := spec.Variables["n6_vlan"].Set(bpfObjects.N6Vlan); err != nil {
		logger.UpfLog.Error("failed to set n6 vlan id", zap.Error(err))
		return err
	}

	if err := spec.Variables["nat_port_min"].Set(NatPortMin); err != nil {
		logger.UpfLog.Error("failed to set nat port min", zap.Error(err))
		return err
	}

	if err := spec.Variables["nat_port_max"].Set(NatPortMax); err != nil {
		logger.UpfLog.Error("failed to set nat port max", zap.Error(err))
		return err
	}

	if opts == nil {
		opts = &ebpf.CollectionOptions{}
	}

	if err := spec.LoadAndAssign(to, opts); err != nil {
		logger.UpfLog.Error("failed to load eBPF program", zap.Error(err))

		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			logger.UpfLog.Debug("verifier log", zap.String("verifier", fmt.Sprintf("%+v", ve)))
		}

		return err
	}

	return nil
}

// mapsRecreatedOnReload lists maps that must NOT be preserved across a reload:
// upf_calls holds program FDs and is repopulated by populateTailCalls. Every
// other datapath map carries session or runtime state and must be preserved.
var mapsRecreatedOnReload = map[string]bool{
	"upf_calls": true,
}

// preservedMaps returns the maps carried across a program reload
// (LoadWithMapReplacements) so their session and runtime state survives a
// global-variable change (NAT/flow-accounting toggle).
//
// Derived from the loaded collection, so a map added to the BPF sources is
// preserved without a second edit here.
func (bpfObjects *BpfObjects) preservedMaps() map[string]*ebpf.Map {
	m := make(map[string]*ebpf.Map)

	v := reflect.ValueOf(bpfObjects.N3N6EntrypointMaps)
	t := v.Type()

	for i := range t.NumField() {
		name, ok := t.Field(i).Tag.Lookup("ebpf")
		if !ok || mapsRecreatedOnReload[name] {
			continue
		}

		field, ok := v.Field(i).Interface().(*ebpf.Map)
		if !ok || field == nil {
			continue
		}

		m[name] = field
	}

	return m
}

// LoadWithMapReplacements reloads the eBPF programs with updated global
// variable values while preserving all existing maps. This allows changing
// settings like flow accounting or masquerade without disrupting subscriber
// sessions, since session state lives in the maps.
//
// The caller must call link.Update() with the new program to atomically
// swap the XDP program on the interface.
func (bpfObjects *BpfObjects) LoadWithMapReplacements() error {
	spec, err := bpfObjects.loadSpec()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}

	// Match the csum_scratch max_entries to the actual CPU count, same as
	// in Load().  Without this the spec still has the placeholder value 1,
	// which conflicts with the existing map that was created with NumCPU().
	if m, ok := spec.Maps["csum_scratch"]; ok {
		m.MaxEntries = uint32(runtime.NumCPU())
	}

	opts := &ebpf.CollectionOptions{
		MapReplacements: bpfObjects.preservedMaps(),
	}

	var newObjects N3N6EntrypointObjects
	if err := bpfObjects.loadAndAssignFromSpec(spec, &newObjects, opts); err != nil {
		logger.UpfLog.Error("failed to load N3/N6 program with map replacements", zap.Error(err))
		return err
	}

	// The prog-array holds program FDs, which are new on this reload, so
	// re-populate the new upf_calls with the new stage programs before the new
	// entry is swapped onto the link.
	if err := populateTailCalls(&newObjects); err != nil {
		return err
	}

	oldPrograms := bpfObjects.N3N6EntrypointPrograms
	oldCalls := bpfObjects.UpfCalls

	bpfObjects.N3N6EntrypointPrograms = newObjects.N3N6EntrypointPrograms
	bpfObjects.UpfCalls = newObjects.UpfCalls
	bpfObjects.N3N6EntrypointVariables = newObjects.N3N6EntrypointVariables

	// Close the cloned map fds from the new objects to avoid fd leaks. These are
	// duplicates of our existing map fds pointing to the same kernel-side maps,
	// so closing them is safe. upf_calls is genuinely new (not preserved) and is
	// now owned by bpfObjects, so swap the old handle in before the wholesale
	// close: the old upf_calls is released and the new one is retained.
	newObjects.UpfCalls = oldCalls
	if err := newObjects.N3N6EntrypointMaps.Close(); err != nil {
		logger.UpfLog.Warn("failed to close cloned map fds", zap.Error(err))
	}

	// The stage programs stay live in the kernel via the new prog-array
	// reference.
	if err := oldPrograms.Close(); err != nil {
		logger.UpfLog.Warn("failed to close old programs", zap.Error(err))
	}

	return nil
}

func (bpfObjects *BpfObjects) Close() error {
	return CloseAllObjects(
		&bpfObjects.N3N6EntrypointObjects,
	)
}

func (bpfObjects *BpfObjects) IsAlreadyNotified(d DataNotification) bool {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	_, ok := bpfObjects.pagingList[d]

	return ok
}

func (bpfObjects *BpfObjects) MarkNotified(d DataNotification) {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	bpfObjects.pagingList[d] = true
}

func (bpfObjects *BpfObjects) ClearNotified(seid uint64, pdrid uint16, qfi uint8) {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	delete(bpfObjects.pagingList, DataNotification{LocalSEID: seid, PdrID: pdrid, QFI: qfi})
}

// ClearNotifiedForSEID removes every paging entry for a session so a released
// SEID leaves none behind.
func (bpfObjects *BpfObjects) ClearNotifiedForSEID(seid uint64) {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	for d := range bpfObjects.pagingList {
		if d.LocalSEID == seid {
			delete(bpfObjects.pagingList, d)
		}
	}
}

func CloseAllObjects(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}

	return nil
}

// profilingMapFromMaps extracts the ProfilingMap field from an
// N3N6EntrypointMaps value using reflection. The field only exists when the
// BPF code was compiled with -DENABLE_PROFILING; reflection lets the rest of
// the package refer to BpfObjects.ProfilingMap unconditionally without a
// compile error when the field is absent in the generated struct.
func profilingMapFromMaps(maps N3N6EntrypointMaps) *ebpf.Map {
	v := reflect.ValueOf(maps)

	f := v.FieldByName("ProfilingMap")
	if !f.IsValid() || f.IsNil() {
		return nil
	}

	m, _ := f.Interface().(*ebpf.Map)

	return m
}
