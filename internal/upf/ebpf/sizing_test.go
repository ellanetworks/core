// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

func TestNewMapSizing(t *testing.T) {
	cases := []struct {
		name                 string
		sessions, nat, flow  uint32
		wantSessions         uint32
		wantNAT, wantFlow    uint32
		wantPDR, wantPerPool uint32
	}{
		{"zero is the default", 0, 0, 0, 2000, 400000, 200000, 2000, 16000},
		{"floor clamps to the minimums", 1000, 0, 0, 1000, 256 << 10, 128 << 10, 1000, 8000},
		{"mid range scales", 20000, 0, 0, 20000, 4_000_000, 2_000_000, 20000, 160000},
		{"ceiling clamps to the maximums", 200000, 0, 0, 200000, 16 << 20, 8 << 20, 200000, 1_600_000},
		{"overrides win", 200000, 1 << 20, 64 << 10, 200000, 1 << 20, 64 << 10, 200000, 1_600_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewMapSizing(tc.sessions, tc.nat, tc.flow)
			if s.MaxSessions != tc.wantSessions || s.NATEntries != tc.wantNAT || s.FlowEntries != tc.wantFlow {
				t.Fatalf("sizing = %+v, want sessions %d nat %d flow %d", s, tc.wantSessions, tc.wantNAT, tc.wantFlow)
			}

			m := s.maxEntries()
			for _, name := range []string{"pdrs_uplink", "pdrs_downlink_ip4", "pdrs_downlink_ip6", "sdf_classifiers", "veth_tunnels"} {
				if m[name] != tc.wantPDR {
					t.Errorf("%s = %d, want %d", name, m[name], tc.wantPDR)
				}
			}

			for _, name := range []string{"qer_windows", "urr_map", "framed_downlink_ip4", "framed_downlink_ip6"} {
				if m[name] != tc.wantPerPool {
					t.Errorf("%s = %d, want %d", name, m[name], tc.wantPerPool)
				}
			}
		})
	}
}

func TestMapSizingApplyCoversBothBuilds(t *testing.T) {
	sizing := NewMapSizing(200000, 0, 0)

	for name, load := range map[string]func() (*ebpf.CollectionSpec, error){
		"xdp": LoadN3N6Entrypoint,
		"tc":  LoadN3N6EntrypointTc,
	} {
		t.Run(name, func(t *testing.T) {
			spec, err := load()
			if err != nil {
				t.Fatalf("load spec: %v", err)
			}

			if err := sizing.apply(spec); err != nil {
				t.Fatalf("apply: %v", err)
			}

			for mapName, want := range sizing.maxEntries() {
				if got := spec.Maps[mapName].MaxEntries; got != want {
					t.Errorf("%s max_entries = %d, want %d", mapName, got, want)
				}
			}

			for _, mapName := range []string{"pdrs_uplink", "pdrs_downlink_ip4", "pdrs_downlink_ip6", "qer_windows", "urr_map", "sdf_classifiers", "veth_tunnels", "framed_downlink_ip4", "framed_downlink_ip6"} {
				if spec.Maps[mapName].Flags&unix.BPF_F_NO_PREALLOC == 0 {
					t.Errorf("%s is preallocated", mapName)
				}
			}

			delete(spec.Maps, "nat_ct")

			if err := sizing.apply(spec); err == nil {
				t.Fatal("apply succeeded with nat_ct missing from the spec")
			}
		})
	}
}

func TestProjectedBytes(t *testing.T) {
	def := NewMapSizing(0, 0, 0).ProjectedBytes()
	if def < 60<<20 || def > 90<<20 {
		t.Errorf("default projection %d MiB, want about 75 MiB", def>>20)
	}

	ceiling := NewMapSizing(200000, 0, 0).ProjectedBytes()
	if ceiling < 3<<30 || ceiling > 4<<30 {
		t.Errorf("ceiling projection %d MiB, want about 3.5 GiB", ceiling>>20)
	}
}

func TestMemoryCheck(t *testing.T) {
	dir := t.TempDir()

	meminfo := filepath.Join(dir, "meminfo")
	selfCgroup := filepath.Join(dir, "cgroup")
	root := filepath.Join(dir, "fs")
	leaf := filepath.Join(root, "system.slice", "ella.service")

	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(path, content string) {
		t.Helper()

		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(meminfo, "MemTotal:       16384000 kB\nMemFree:  1 kB\n")
	write(selfCgroup, "0::/system.slice/ella.service\n")
	write(filepath.Join(leaf, "memory.max"), "max\n")
	write(filepath.Join(root, "system.slice", "memory.max"), "8589934592\n")

	src := MemorySources{Meminfo: meminfo, SelfCgroup: selfCgroup, CgroupRoot: root}

	limit, warn := NewMapSizing(0, 0, 0).MemoryCheck(src)
	if limit != 8<<30 {
		t.Fatalf("limit = %d, want the parent cgroup's 8 GiB", limit)
	}

	if warn {
		t.Error("default sizing warned on 8 GiB")
	}

	if _, warn := NewMapSizing(200000, 0, 0).MemoryCheck(src); !warn {
		t.Error("ceiling sizing did not warn on 8 GiB")
	}

	write(filepath.Join(root, "system.slice", "memory.max"), "max\n")

	if limit, _ := NewMapSizing(0, 0, 0).MemoryCheck(src); limit != 16384000<<10 {
		t.Errorf("limit = %d, want MemTotal without a cgroup limit", limit)
	}

	if limit, warn := NewMapSizing(0, 0, 0).MemoryCheck(MemorySources{}); limit != 0 || warn {
		t.Errorf("missing sources gave limit %d warn %v", limit, warn)
	}
}

func TestWrapCreateError(t *testing.T) {
	s := NewMapSizing(200000, 0, 0)

	err := s.wrapCreateError(unix.ENOMEM)
	if !errors.Is(err, unix.ENOMEM) || !strings.Contains(err.Error(), "datapath.max-sessions") {
		t.Errorf("ENOMEM wrapped as %q", err)
	}

	other := errors.New("verifier")
	if got := s.wrapCreateError(other); got != other {
		t.Errorf("unrelated error rewrapped as %q", got)
	}
}

func TestRetryTransientENOMEM(t *testing.T) {
	calls := 0

	err := retryTransientENOMEM(func() error {
		calls++
		if calls < 3 {
			return unix.ENOMEM
		}

		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err %v after %d calls, want success on the third", err, calls)
	}

	calls = 0

	err = retryTransientENOMEM(func() error {
		calls++
		return unix.ENOMEM
	})
	if !errors.Is(err, unix.ENOMEM) || calls != 1+transientENOMEMRetries {
		t.Fatalf("err %v after %d calls, want ENOMEM after %d", err, calls, 1+transientENOMEMRetries)
	}

	calls = 0

	err = retryTransientENOMEM(func() error {
		calls++
		return unix.E2BIG
	})
	if !errors.Is(err, unix.E2BIG) || calls != 1 {
		t.Fatalf("err %v after %d calls, want E2BIG without retry", err, calls)
	}
}
