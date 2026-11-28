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
	"maps"
	"reflect"
	"slices"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// BPF_CFLAGS selects optional instrumentation: -DENABLE_LOG for debug output
// on the trace pipe, -DENABLE_PROFILING for per-packet latency in
// profiling_map, read back by ReadProfilingStats. Profiling costs ~600-900 ns
// per sample.

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3N6Entrypoint bpf/n3n6_bpf.c -- -I. -O2 -Wall -Werror -g
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3N6EntrypointTc bpf/n3n6_bpf.c -- -DCTX_TC -I. -O2 -Wall -Werror -g
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf -type urr_first_packet_args UrrFirstPacket bpf/urr_first_packet_bpf.c -- -I. -O2 -Wall -Werror -g

// Masquerade source-port range, held below the kernel's default ephemeral
// floor so an allocation cannot name a port a host socket is using.
const (
	NatPortMin uint16 = 1024
	NatPortMax uint16 = 32767
)

// MaxDlBufferPkt is the largest packet the datapath will capture, matching DL_BUFFER_MAX_PKT.
const MaxDlBufferPkt = 9000

type DataNotification struct {
	LocalSEID uint64
	PdrID     uint16
	QFI       uint8
}

// DlBufferHeader is the Go representation of struct dl_buffer_hdr.
type DlBufferHeader struct {
	LocalSEID uint64
	PdrID     uint16
	Len       uint16
	QFI       uint8
	Family    uint8 // 4 or 6
	Pad       uint16
}

// DlBufferCounters mirrors struct dl_buffer_counters.
type DlBufferCounters struct {
	Captured uint64
	RingFull uint64
	TooLarge uint64
	GSO      uint64
}

const MaxFragTxPkt = 9216

// RSEvent is the Go representation of struct rs_event emitted by the N3 XDP
// program when a Router Solicitation is detected inside a GTP-U packet.
type RSEvent struct {
	TEID    uint32
	UEIPv6  [16]byte // struct in6_addr — UE source IPv6 address
	DstIPv6 [16]byte
	MsgLen  uint16
	Msg     [RSMsgMax]byte
}

const RSMsgMax = 1240

type ErrorIndication struct {
	TEID     uint32
	PeerAddr [16]byte
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
	N3RoutingIndex   uint32
	N6RoutingIndex   uint32
	N3Vlan           uint32
	N6Vlan           uint32

	NetworkInstancesEnabled bool

	settingsMu sync.Mutex

	pagingMu   sync.Mutex
	pagingList map[DataNotification]bool

	occupancyMu sync.Mutex
	occupancy   map[string]int

	urrFirstPacket UrrFirstPacketPrograms

	interfacesMu sync.Mutex
	interfaces   map[uint32]*interfacePrograms

	Programs *ProgramBarrier

	Sizing MapSizing
}

func NewBpfObjects(flowact bool, masquerade bool, n3ifindex int, n6ifindex int, n3vlan uint32, n6vlan uint32) *BpfObjects {
	return &BpfObjects{
		FlowAccounting:          flowact,
		Masquerade:              masquerade,
		N3InterfaceIndex:        uint32(n3ifindex),
		N6InterfaceIndex:        uint32(n6ifindex),
		N3RoutingIndex:          uint32(n3ifindex),
		N6RoutingIndex:          uint32(n6ifindex),
		N3Vlan:                  n3vlan,
		N6Vlan:                  n6vlan,
		NetworkInstancesEnabled: true,
		pagingList:              make(map[DataNotification]bool),
		occupancy:               make(map[string]int),
	}
}

// Tail-call indices in the upf_calls PROG_ARRAY; must match UPF_CALL_* in
// bpf/n3n6_bpf.c.
const (
	upfCallUplink      = 0
	upfCallDownlink    = 1
	upfCallGtpuControl = 2
	upfCallLocalSwitch = 3
)

// populateTailCalls inserts the stage programs into the upf_calls PROG_ARRAY so
// upf_entry_func can tail-call them. Must run after every load/reload: the array
// holds program FDs, which are new on each load.
func populateTailCalls(calls *ebpf.Map, uplink, downlink, gtpuControl, localSwitch *ebpf.Program) error {
	if err := calls.Put(uint32(upfCallUplink), uplink); err != nil {
		return fmt.Errorf("put uplink stage in upf_calls: %w", err)
	}

	if err := calls.Put(uint32(upfCallDownlink), downlink); err != nil {
		return fmt.Errorf("put downlink stage in upf_calls: %w", err)
	}

	if err := calls.Put(uint32(upfCallGtpuControl), gtpuControl); err != nil {
		return fmt.Errorf("put gtpu control stage in upf_calls: %w", err)
	}

	if err := calls.Put(uint32(upfCallLocalSwitch), localSwitch); err != nil {
		return fmt.Errorf("put local switch stage in upf_calls: %w", err)
	}

	return nil
}

func populateEntrypointTailCalls(objs *N3N6EntrypointObjects) error {
	return populateTailCalls(objs.UpfCalls, objs.UpfUplinkFunc, objs.UpfDownlinkFunc, objs.UpfGtpuControlFunc, objs.UpfLocalSwitchFunc)
}

func (bpfObjects *BpfObjects) loadSpec() (*ebpf.CollectionSpec, error) {
	if bpfObjects.UseTCX {
		return LoadN3N6EntrypointTc()
	}

	return LoadN3N6Entrypoint()
}

// sizeCPUScratchMaps sets max_entries of the per-CPU-indexed scratch ARRAYs to the number of possible CPUs.
func sizeCPUScratchMaps(spec *ebpf.CollectionSpec) error {
	cpus, err := ebpf.PossibleCPU()
	if err != nil {
		return fmt.Errorf("get possible CPUs: %w", err)
	}

	for _, name := range []string{"csum_scratch", "dl_buffer_scratch", "frag_tx_scratch"} {
		if m, ok := spec.Maps[name]; ok {
			m.MaxEntries = uint32(cpus)
		}
	}

	return nil
}

func (bpfObjects *BpfObjects) Load() error {
	n3n6Spec, err := bpfObjects.loadSpec()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}

	if err := sizeCPUScratchMaps(n3n6Spec); err != nil {
		logger.UpfLog.Error("failed to size scratch maps", zap.Error(err))
		return err
	}

	bpfObjects.Sizing = bpfObjects.Sizing.orDefault()

	if err := bpfObjects.Sizing.apply(n3n6Spec); err != nil {
		return err
	}

	cpus, err := ebpf.PossibleCPU()
	if err != nil {
		return fmt.Errorf("get possible CPUs: %w", err)
	}

	projected := bpfObjects.Sizing.ProjectedBytes(n3n6Spec, cpus)

	if limit, warn := MemoryCheck(HostMemory, projected); warn {
		logger.UpfLog.Warn("datapath maps reserve more than a quarter of the memory available to Ella Core",
			zap.Uint32("max_sessions", bpfObjects.Sizing.MaxSessions),
			zap.Uint64("projected_bytes", projected),
			zap.Uint64("memory_limit_bytes", limit))
	}

	if bpfObjects.Sizing.clamped() {
		logger.UpfLog.Warn("NAT and flow tables are at their maximum size, so each session gets less than its nominal share",
			zap.Uint32("max_sessions", bpfObjects.Sizing.MaxSessions),
			zap.Uint32("nat_connections_per_session", bpfObjects.Sizing.natConnectionsPerSession()),
			zap.Uint32("flows_per_session", bpfObjects.Sizing.flowsPerSession()))
	}

	if err := bpfObjects.loadAndAssignFromSpec(n3n6Spec, &bpfObjects.N3N6EntrypointObjects, nil); err != nil {
		logger.UpfLog.Error("failed to load N3/N6 program", zap.Error(err))
		return bpfObjects.Sizing.wrapCreateError(err, projected)
	}

	logger.UpfLog.Info("datapath maps sized",
		zap.Uint32("max_sessions", bpfObjects.Sizing.MaxSessions),
		zap.Uint32("nat_ct_entries", bpfObjects.Sizing.NATEntries),
		zap.Uint32("flow_stats_entries", bpfObjects.Sizing.FlowEntries),
		zap.Uint64("memlock_bytes", mapsMemlock(bpfObjects.N3N6EntrypointMaps)))

	if err := populateEntrypointTailCalls(&bpfObjects.N3N6EntrypointObjects); err != nil {
		logger.UpfLog.Error("failed to populate tail-call program array", zap.Error(err))
		return err
	}

	if err := bpfObjects.loadUrrFirstPacket(); err != nil {
		logger.UpfLog.Error("failed to load URR first packet program", zap.Error(err))
		return err
	}

	// Populate the optional profiling map from the embedded maps struct. The
	// field only exists in N3N6EntrypointMaps when compiled with
	// -DENABLE_PROFILING; reflection lets us check without a hard reference.
	bpfObjects.ProfilingMap = profilingMapFromMaps(bpfObjects.N3N6EntrypointMaps)

	if bpfObjects.Programs == nil {
		programs, err := NewProgramBarrier()
		if err != nil {
			logger.UpfLog.Error("failed to create datapath program barrier", zap.Error(err))
			return err
		}

		bpfObjects.Programs = programs
	}

	return nil
}

func (bpfObjects *BpfObjects) loadUrrFirstPacket() error {
	spec, err := LoadUrrFirstPacket()
	if err != nil {
		return fmt.Errorf("load URR first packet spec: %w", err)
	}

	spec.Maps["urr_map"].MaxEntries = bpfObjects.UrrMap.MaxEntries()

	opts := &ebpf.CollectionOptions{
		MapReplacements: map[string]*ebpf.Map{"urr_map": bpfObjects.UrrMap},
	}

	if err := spec.LoadAndAssign(&bpfObjects.urrFirstPacket, opts); err != nil {
		return fmt.Errorf("load URR first packet program: %w", err)
	}

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

	if err := spec.Variables["n3_routing_ifindex"].Set(bpfObjects.N3RoutingIndex); err != nil {
		return fmt.Errorf("set n3 routing interface index: %w", err)
	}

	if err := spec.Variables["n6_routing_ifindex"].Set(bpfObjects.N6RoutingIndex); err != nil {
		return fmt.Errorf("set n6 routing interface index: %w", err)
	}

	if err := spec.Variables["n3_vlan"].Set(bpfObjects.N3Vlan); err != nil {
		logger.UpfLog.Error("failed to set n3 vlan id", zap.Error(err))
		return err
	}

	if err := spec.Variables["n6_vlan"].Set(bpfObjects.N6Vlan); err != nil {
		logger.UpfLog.Error("failed to set n6 vlan id", zap.Error(err))
		return err
	}

	if err := spec.Variables["network_instances_enabled"].Set(bpfObjects.NetworkInstancesEnabled); err != nil {
		return fmt.Errorf("set network instances enabled: %w", err)
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
	release, err := bpfObjects.ReloadPrograms()
	if err != nil {
		return err
	}

	release()

	return nil
}

type interfacePrograms struct {
	UpfEntryFunc       *ebpf.Program `ebpf:"upf_entry_func"`
	UpfUplinkFunc      *ebpf.Program `ebpf:"upf_uplink_func"`
	UpfDownlinkFunc    *ebpf.Program `ebpf:"upf_downlink_func"`
	UpfGtpuControlFunc *ebpf.Program `ebpf:"upf_gtpu_control_func"`
	UpfLocalSwitchFunc *ebpf.Program `ebpf:"upf_local_switch_func"`
	UpfCalls           *ebpf.Map     `ebpf:"upf_calls"`
}

func (p *interfacePrograms) close() {
	for _, prog := range []*ebpf.Program{p.UpfEntryFunc, p.UpfUplinkFunc, p.UpfDownlinkFunc, p.UpfGtpuControlFunc, p.UpfLocalSwitchFunc} {
		if prog != nil {
			_ = prog.Close()
		}
	}

	if p.UpfCalls != nil {
		_ = p.UpfCalls.Close()
	}
}

func (bpfObjects *BpfObjects) loadSharingMaps(attachIfindex uint32, to any) error {
	spec, err := bpfObjects.loadSpec()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}

	if err := sizeCPUScratchMaps(spec); err != nil {
		logger.UpfLog.Error("failed to size scratch maps", zap.Error(err))
		return err
	}

	if err := bpfObjects.Sizing.orDefault().apply(spec); err != nil {
		return err
	}

	if err := spec.Variables["attach_ifindex"].Set(attachIfindex); err != nil {
		return fmt.Errorf("set attach interface index: %w", err)
	}

	opts := &ebpf.CollectionOptions{
		MapReplacements: bpfObjects.preservedMaps(),
	}

	if err := bpfObjects.loadAndAssignFromSpec(spec, to, opts); err != nil {
		logger.UpfLog.Error("failed to load N3/N6 program with map replacements", zap.Error(err))
		return err
	}

	return nil
}

func (bpfObjects *BpfObjects) reloadEntrypoint() (*N3N6EntrypointObjects, error) {
	var newObjects N3N6EntrypointObjects
	if err := bpfObjects.loadSharingMaps(0, &newObjects); err != nil {
		return nil, err
	}

	// The prog-array holds program FDs, which are new on this reload, so
	// re-populate the new upf_calls with the new stage programs before the new
	// entry is swapped onto the link.
	if err := populateEntrypointTailCalls(&newObjects); err != nil {
		closeSharingMaps(&newObjects)

		return nil, err
	}

	return &newObjects, nil
}

func (bpfObjects *BpfObjects) loadInterface(ifindex uint32) (*interfacePrograms, error) {
	var programs interfacePrograms
	if err := bpfObjects.loadSharingMaps(ifindex, &programs); err != nil {
		return nil, fmt.Errorf("load datapath for interface %d: %w", ifindex, err)
	}

	if err := populateTailCalls(programs.UpfCalls, programs.UpfUplinkFunc, programs.UpfDownlinkFunc,
		programs.UpfGtpuControlFunc, programs.UpfLocalSwitchFunc); err != nil {
		programs.close()

		return nil, err
	}

	return &programs, nil
}

func (bpfObjects *BpfObjects) loadInterfaces(ifindexes []uint32) (map[uint32]*interfacePrograms, error) {
	var (
		mu     sync.Mutex
		group  errgroup.Group
		loaded = make(map[uint32]*interfacePrograms, len(ifindexes))
	)

	for _, ifindex := range ifindexes {
		group.Go(func() error {
			programs, err := bpfObjects.loadInterface(ifindex)
			if err != nil {
				return err
			}

			mu.Lock()
			loaded[ifindex] = programs
			mu.Unlock()

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		for _, programs := range loaded {
			programs.close()
		}

		return nil, err
	}

	return loaded, nil
}

func closeSharingMaps(objs *N3N6EntrypointObjects) {
	if err := objs.N3N6EntrypointMaps.Close(); err != nil {
		logger.UpfLog.Warn("failed to close cloned map fds", zap.Error(err))
	}

	if err := objs.N3N6EntrypointPrograms.Close(); err != nil {
		logger.UpfLog.Warn("failed to close programs", zap.Error(err))
	}
}

func (bpfObjects *BpfObjects) InterfaceEntry(ifindex int) (*ebpf.Program, bool, error) {
	bpfObjects.interfacesMu.Lock()
	defer bpfObjects.interfacesMu.Unlock()

	if programs, ok := bpfObjects.interfaces[uint32(ifindex)]; ok {
		return programs.UpfEntryFunc, false, nil
	}

	programs, err := bpfObjects.loadInterface(uint32(ifindex))
	if err != nil {
		return nil, false, err
	}

	if bpfObjects.interfaces == nil {
		bpfObjects.interfaces = make(map[uint32]*interfacePrograms)
	}

	bpfObjects.interfaces[uint32(ifindex)] = programs

	return programs.UpfEntryFunc, true, nil
}

func (bpfObjects *BpfObjects) ReleaseInterface(ifindex int) {
	bpfObjects.interfacesMu.Lock()
	defer bpfObjects.interfacesMu.Unlock()

	programs, ok := bpfObjects.interfaces[uint32(ifindex)]
	if !ok {
		return
	}

	programs.close()
	delete(bpfObjects.interfaces, uint32(ifindex))
}

func (bpfObjects *BpfObjects) ReloadPrograms() (func(), error) {
	bpfObjects.interfacesMu.Lock()
	defer bpfObjects.interfacesMu.Unlock()

	var (
		group      errgroup.Group
		newObjects *N3N6EntrypointObjects
		reloaded   map[uint32]*interfacePrograms
	)

	group.Go(func() error {
		var err error

		newObjects, err = bpfObjects.reloadEntrypoint()

		return err
	})

	group.Go(func() error {
		var err error

		reloaded, err = bpfObjects.loadInterfaces(slices.Collect(maps.Keys(bpfObjects.interfaces)))

		return err
	})

	if err := group.Wait(); err != nil {
		if newObjects != nil {
			closeSharingMaps(newObjects)
		}

		for _, programs := range reloaded {
			programs.close()
		}

		return nil, err
	}

	oldInterfaces := bpfObjects.interfaces
	bpfObjects.interfaces = reloaded

	oldPrograms := bpfObjects.N3N6EntrypointPrograms
	oldCalls := bpfObjects.UpfCalls

	bpfObjects.N3N6EntrypointPrograms = newObjects.N3N6EntrypointPrograms
	bpfObjects.UpfCalls = newObjects.UpfCalls
	bpfObjects.N3N6EntrypointVariables = newObjects.N3N6EntrypointVariables

	release := func() {
		for _, programs := range oldInterfaces {
			programs.close()
		}

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
	}

	return release, nil
}

func (bpfObjects *BpfObjects) Close() error {
	bpfObjects.interfacesMu.Lock()
	for _, programs := range bpfObjects.interfaces {
		programs.close()
	}

	bpfObjects.interfaces = nil
	bpfObjects.interfacesMu.Unlock()

	err := CloseAllObjects(
		&bpfObjects.urrFirstPacket,
		&bpfObjects.N3N6EntrypointObjects,
	)

	if bpfObjects.Programs != nil {
		err = errors.Join(err, bpfObjects.Programs.Close())
		bpfObjects.Programs = nil
	}

	return err
}

// SetBufferVethIfindex names the veth re-injected packets arrive on. Zero clears it.
func (bpfObjects *BpfObjects) SetBufferVethIfindex(vethIfindex int) error {
	return bpfObjects.updateRuntimeSettings(func(settings *N3N6EntrypointRuntimeSettings) {
		settings.BufferVethIfindex = uint32(vethIfindex)
	})
}

func (bpfObjects *BpfObjects) SetReasmVethIfindex(vethIfindex int) error {
	return bpfObjects.updateRuntimeSettings(func(settings *N3N6EntrypointRuntimeSettings) {
		settings.ReasmVethIfindex = uint32(vethIfindex)
	})
}

func (bpfObjects *BpfObjects) SetFragTxEnabled(enabled bool) error {
	return bpfObjects.updateRuntimeSettings(func(settings *N3N6EntrypointRuntimeSettings) {
		settings.FragTxEnabled = 0
		if enabled {
			settings.FragTxEnabled = 1
		}
	})
}

func (bpfObjects *BpfObjects) updateRuntimeSettings(update func(*N3N6EntrypointRuntimeSettings)) error {
	bpfObjects.settingsMu.Lock()
	defer bpfObjects.settingsMu.Unlock()

	var settings N3N6EntrypointRuntimeSettings
	if err := bpfObjects.RuntimeSettingsMap.Lookup(uint32(0), &settings); err != nil {
		return fmt.Errorf("read runtime settings: %w", err)
	}

	update(&settings)

	if err := bpfObjects.RuntimeSettingsMap.Put(uint32(0), &settings); err != nil {
		return fmt.Errorf("write runtime settings: %w", err)
	}

	return nil
}

// GetDlBufferCounters sums the per-CPU capture counters.
func (bpfObjects *BpfObjects) GetDlBufferCounters() (DlBufferCounters, bool) {
	var (
		perCPU []N3N6EntrypointDlBufferCounters
		total  DlBufferCounters
	)

	if err := bpfObjects.DlBufferCountersMap.Lookup(uint32(0), &perCPU); err != nil {
		logger.UpfLog.Warn("failed to fetch dl buffer counters", zap.Error(err))
		return total, false
	}

	for _, c := range perCPU {
		total.Captured += c.Captured
		total.RingFull += c.RingFull
		total.TooLarge += c.TooLarge
		total.GSO += c.Gso
	}

	return total, true
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

// Every QFI, because a caller only knows the PDR's current one and a policy
// change moves it: deleting the exact triple strands the entry written under the
// previous QFI until the session is deleted.
func (bpfObjects *BpfObjects) ClearNotified(seid uint64, pdrid uint16) {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	for d := range bpfObjects.pagingList {
		if d.LocalSEID == seid && d.PdrID == pdrid {
			delete(bpfObjects.pagingList, d)
		}
	}
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
