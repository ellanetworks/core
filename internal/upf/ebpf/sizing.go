// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"
	"fmt"
	"io/fs"
	"math/bits"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

const (
	DefaultMaxSessions = 2000

	maxTableEntries  = 1 << 24
	MaxSessionsLimit = maxTableEntries / MaxSessionQERs

	natEntriesPerConnection = 2
	natEntriesPerSession    = 200
	flowEntriesPerSession   = 100
	natEntriesMin           = 256 << 10
	flowEntriesMin          = 128 << 10
	natEntriesMax           = maxTableEntries
	flowEntriesMax          = 8 << 20

	perSessionPoolFactor = 8

	htabBucketBytes        = 16
	htabElemHeaderBytes    = 48
	memoryWarnRatioPercent = 25
)

type MapSizing struct {
	MaxSessions uint32
	NATEntries  uint32
	FlowEntries uint32
}

func NewMapSizing(maxSessions uint32) MapSizing {
	if maxSessions == 0 {
		maxSessions = DefaultMaxSessions
	}

	return MapSizing{
		MaxSessions: maxSessions,
		NATEntries:  clampEntries(uint64(maxSessions)*natEntriesPerSession, natEntriesMin, natEntriesMax),
		FlowEntries: clampEntries(uint64(maxSessions)*flowEntriesPerSession, flowEntriesMin, flowEntriesMax),
	}
}

func clampEntries(v uint64, lo, hi uint32) uint32 {
	return uint32(min(max(v, uint64(lo)), uint64(hi)))
}

func (s MapSizing) orDefault() MapSizing {
	if s.MaxSessions == 0 {
		return NewMapSizing(0)
	}

	return s
}

func (s MapSizing) maxEntries() map[string]uint32 {
	pool := s.MaxSessions * perSessionPoolFactor

	return map[string]uint32{
		"pdrs_uplink":         s.MaxSessions,
		"pdrs_downlink_ip4":   s.MaxSessions,
		"pdrs_downlink_ip6":   s.MaxSessions,
		"sdf_classifiers":     s.MaxSessions,
		"veth_tunnels":        s.MaxSessions,
		"qer_windows":         s.MaxSessions * MaxSessionQERs,
		"urr_map":             pool,
		"framed_downlink_ip4": pool,
		"framed_downlink_ip6": pool,
		"nat_ct":              s.NATEntries,
		"flow_stats":          s.FlowEntries,
	}
}

func (s MapSizing) apply(spec *ebpf.CollectionSpec) error {
	for name, n := range s.maxEntries() {
		m, ok := spec.Maps[name]
		if !ok {
			return fmt.Errorf("sized map %q is missing from the datapath spec", name)
		}

		m.MaxEntries = n
	}

	return nil
}

func (s MapSizing) clamped() bool {
	return uint64(s.NATEntries) < uint64(s.MaxSessions)*natEntriesPerSession ||
		uint64(s.FlowEntries) < uint64(s.MaxSessions)*flowEntriesPerSession
}

func (s MapSizing) natConnectionsPerSession() uint32 {
	return s.NATEntries / natEntriesPerConnection / s.MaxSessions
}

func (s MapSizing) flowsPerSession() uint32 {
	return s.FlowEntries / s.MaxSessions
}

func (s MapSizing) ProjectedBytes(spec *ebpf.CollectionSpec, possibleCPUs int) uint64 {
	total := uint64(0)

	for name, n := range s.maxEntries() {
		if m, ok := spec.Maps[name]; ok {
			total += startupBytes(m, n, uint64(possibleCPUs))
		}
	}

	return total
}

func startupBytes(m *ebpf.MapSpec, entries uint32, cpus uint64) uint64 {
	switch m.Type {
	case ebpf.Hash, ebpf.LRUHash, ebpf.PerCPUHash, ebpf.LRUCPUHash:
	default:
		return 0
	}

	total := roundUpPow2(entries) * htabBucketBytes
	if m.Flags&unix.BPF_F_NO_PREALLOC != 0 {
		return total
	}

	lru := m.Type == ebpf.LRUHash || m.Type == ebpf.LRUCPUHash
	percpu := m.Type == ebpf.PerCPUHash || m.Type == ebpf.LRUCPUHash
	value := roundUp8(m.ValueSize)

	elem := htabElemHeaderBytes + roundUp8(m.KeySize)
	if percpu {
		elem += 8
	} else {
		elem += value
	}

	n := uint64(entries)
	if !lru && !percpu {
		n += cpus
	}

	total += elem * n
	if percpu {
		total += value * cpus * uint64(entries)
	}

	return total
}

func roundUp8(n uint32) uint64 {
	return (uint64(n) + 7) &^ 7
}

func roundUpPow2(n uint32) uint64 {
	if n <= 1 {
		return 1
	}

	return 1 << bits.Len32(n-1)
}

func (s MapSizing) wrapCreateError(err error, projected uint64) error {
	if !errors.Is(err, unix.ENOMEM) && !errors.Is(err, unix.E2BIG) {
		return err
	}

	return fmt.Errorf("create datapath maps for %d sessions (nat_ct %d entries, flow_stats %d entries, about %d MiB): %w; raise the memory limit",
		s.MaxSessions, s.NATEntries, s.FlowEntries, projected>>20, err)
}

var HostMemory = os.DirFS("/")

const (
	meminfoPath    = "proc/meminfo"
	selfCgroupPath = "proc/self/cgroup"
	cgroupRootPath = "sys/fs/cgroup"
)

func MemoryCheck(host fs.FS, projected uint64) (limit uint64, warn bool) {
	limit = availableMemoryLimit(host)
	if limit == 0 {
		return 0, false
	}

	return limit, projected*100 > limit*memoryWarnRatioPercent
}

func availableMemoryLimit(host fs.FS) uint64 {
	limit := memTotal(host)

	if c := cgroupMemoryMax(host); c != 0 && (limit == 0 || c < limit) {
		limit = c
	}

	return limit
}

func memTotal(host fs.FS) uint64 {
	raw, err := fs.ReadFile(host, meminfoPath)
	if err != nil {
		return 0
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}

		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}

		return kb << 10
	}

	return 0
}

func cgroupMemoryMax(host fs.FS) uint64 {
	raw, err := fs.ReadFile(host, selfCgroupPath)
	if err != nil {
		return 0
	}

	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		rel, ok := strings.CutPrefix(line, "0::")
		if !ok {
			continue
		}

		limit := uint64(0)

		for dir := path.Clean("/" + rel); ; dir = path.Dir(dir) {
			if n := cgroupDirMemoryMax(host, dir); n != 0 && (limit == 0 || n < limit) {
				limit = n
			}

			if dir == "/" {
				return limit
			}
		}
	}

	return 0
}

func cgroupDirMemoryMax(host fs.FS, dir string) uint64 {
	memoryMax, err := fs.ReadFile(host, path.Join(cgroupRootPath, dir, "memory.max"))
	if err != nil {
		return 0
	}

	n, err := strconv.ParseUint(strings.TrimSpace(string(memoryMax)), 10, 64)
	if err != nil {
		return 0
	}

	return n
}

func mapsMemlock(maps N3N6EntrypointMaps) uint64 {
	total := uint64(0)

	v := reflect.ValueOf(maps)
	for i := range v.NumField() {
		m, ok := v.Field(i).Interface().(*ebpf.Map)
		if !ok || m == nil {
			continue
		}

		total += fdinfoMemlock(m.FD())
	}

	return total
}

func fdinfoMemlock(fd int) uint64 {
	raw, err := os.ReadFile("/proc/self/fdinfo/" + strconv.Itoa(fd))
	if err != nil {
		return 0
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, "memlock:"); ok {
			n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return 0
			}

			return n
		}
	}

	return 0
}
