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

	natEntriesPerSession  = 200
	flowEntriesPerSession = 100
	natEntriesMin         = 256 << 10
	flowEntriesMin        = 128 << 10
	natEntriesMax         = 16 << 20
	flowEntriesMax        = 8 << 20

	perSessionPoolFactor = 8

	natBytesPerEntry       = 112
	flowBytesPerEntry      = 163
	hashBucketBytes        = 16
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

func (s MapSizing) ProjectedBytes() uint64 {
	total := uint64(s.NATEntries)*natBytesPerEntry + uint64(s.FlowEntries)*flowBytesPerEntry

	for name, n := range s.maxEntries() {
		if name == "nat_ct" || name == "flow_stats" {
			continue
		}

		total += roundUpPow2(n) * hashBucketBytes
	}

	return total
}

func roundUpPow2(n uint32) uint64 {
	if n <= 1 {
		return 1
	}

	return 1 << bits.Len32(n-1)
}

func (s MapSizing) wrapCreateError(err error) error {
	if !errors.Is(err, unix.ENOMEM) && !errors.Is(err, unix.E2BIG) {
		return err
	}

	return fmt.Errorf("create datapath maps for %d sessions (nat_ct %d entries, flow_stats %d entries, about %d MiB): %w; raise the memory limit",
		s.MaxSessions, s.NATEntries, s.FlowEntries, s.ProjectedBytes()>>20, err)
}

var HostMemory = os.DirFS("/")

const (
	meminfoPath    = "proc/meminfo"
	selfCgroupPath = "proc/self/cgroup"
	cgroupRootPath = "sys/fs/cgroup"
)

func (s MapSizing) MemoryCheck(host fs.FS) (limit uint64, warn bool) {
	limit = availableMemoryLimit(host)
	if limit == 0 {
		return 0, false
	}

	return limit, s.ProjectedBytes()*100 > limit*memoryWarnRatioPercent
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

		memoryMax, err := fs.ReadFile(host, path.Join(cgroupRootPath, rel, "memory.max"))
		if err != nil {
			return 0
		}

		n, err := strconv.ParseUint(strings.TrimSpace(string(memoryMax)), 10, 64)
		if err != nil {
			return 0
		}

		return n
	}

	return 0
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
