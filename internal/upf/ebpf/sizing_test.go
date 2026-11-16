// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

func TestNewMapSizing(t *testing.T) {
	cases := []struct {
		name                 string
		sessions             uint32
		wantSessions         uint32
		wantNAT, wantFlow    uint32
		wantPDR, wantPerPool uint32
	}{
		{"zero is the default", 0, 2000, 400000, 200000, 2000, 16000},
		{"floor clamps to the minimums", 1000, 1000, 256 << 10, 128 << 10, 1000, 8000},
		{"mid range scales", 20000, 20000, 4_000_000, 2_000_000, 20000, 160000},
		{"ceiling clamps to the maximums", 200000, 200000, 16 << 20, 8 << 20, 200000, 1_600_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewMapSizing(tc.sessions)
			if s.MaxSessions != tc.wantSessions || s.NATEntries != tc.wantNAT || s.FlowEntries != tc.wantFlow {
				t.Fatalf("sizing = %+v, want sessions %d nat %d flow %d", s, tc.wantSessions, tc.wantNAT, tc.wantFlow)
			}

			m := s.maxEntries()
			for _, name := range []string{"pdrs_uplink", "pdrs_downlink_ip4", "pdrs_downlink_ip6", "sdf_classifiers", "veth_tunnels"} {
				if m[name] != tc.wantPDR {
					t.Errorf("%s = %d, want %d", name, m[name], tc.wantPDR)
				}
			}

			if want := tc.wantSessions * MaxSessionQERs; m["qer_windows"] != want {
				t.Errorf("qer_windows = %d, want %d: every session can hold %d QERs", m["qer_windows"], want, MaxSessionQERs)
			}

			for _, name := range []string{"urr_map", "framed_downlink_ip4", "framed_downlink_ip6"} {
				if m[name] != tc.wantPerPool {
					t.Errorf("%s = %d, want %d", name, m[name], tc.wantPerPool)
				}
			}
		})
	}
}

func TestMapSizingApplyCoversBothBuilds(t *testing.T) {
	sizing := NewMapSizing(200000)

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
	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	def := NewMapSizing(0).ProjectedBytes(spec, 4)
	if def < 60<<20 || def > 90<<20 {
		t.Errorf("default projection %d MiB, want about 77 MiB", def>>20)
	}

	ceiling := NewMapSizing(MaxSessionsLimit).ProjectedBytes(spec, 4)
	if ceiling < 3<<30 || ceiling > 4<<30 {
		t.Errorf("ceiling projection %d MiB, want about 3.6 GiB", ceiling>>20)
	}
}

func TestSessionLimitKeepsEveryMapWithinTheTableMaximum(t *testing.T) {
	for name, n := range NewMapSizing(MaxSessionsLimit).maxEntries() {
		if n > maxTableEntries {
			t.Errorf("%s = %d entries at %d sessions, above %d", name, n, MaxSessionsLimit, maxTableEntries)
		}
	}

	if n := NewMapSizing(MaxSessionsLimit + 1).maxEntries()["qer_windows"]; n <= maxTableEntries {
		t.Errorf("qer_windows = %d at %d sessions, want the limit to be the largest count that fits", n, MaxSessionsLimit+1)
	}
}

func TestClampedShare(t *testing.T) {
	cases := []struct {
		sessions          uint32
		clamped           bool
		connections, flow uint32
	}{
		{2000, false, 100, 100},
		{80000, false, 100, 100},
		{100000, true, 83, 83},
		{MaxSessionsLimit, true, 16, 16},
	}

	for _, tc := range cases {
		s := NewMapSizing(tc.sessions)
		if s.clamped() != tc.clamped || s.natConnectionsPerSession() != tc.connections || s.flowsPerSession() != tc.flow {
			t.Errorf("%d sessions: clamped %v, %d connections, %d flows per session; want %v, %d, %d",
				tc.sessions, s.clamped(), s.natConnectionsPerSession(), s.flowsPerSession(), tc.clamped, tc.connections, tc.flow)
		}
	}
}

func TestMemoryCheck(t *testing.T) {
	host := fstest.MapFS{
		"proc/meminfo":     {Data: []byte("MemTotal:       16384000 kB\nMemFree:  1 kB\n")},
		"proc/self/cgroup": {Data: []byte("0::/system.slice/ella.service\n")},
		"sys/fs/cgroup/system.slice/ella.service/memory.max": {Data: []byte("8589934592\n")},
	}

	limit, warn := MemoryCheck(host, 1<<30)
	if limit != 8<<30 {
		t.Fatalf("limit = %d, want the service cgroup's 8 GiB", limit)
	}

	if warn {
		t.Error("1 GiB warned on 8 GiB")
	}

	if _, warn := MemoryCheck(host, 3<<30); !warn {
		t.Error("3 GiB did not warn on 8 GiB")
	}

	host["sys/fs/cgroup/system.slice/ella.service/memory.max"] = &fstest.MapFile{Data: []byte("max\n")}
	host["sys/fs/cgroup/system.slice/memory.max"] = &fstest.MapFile{Data: []byte("1073741824\n")}

	if limit, _ := MemoryCheck(host, 0); limit != 1<<30 {
		t.Errorf("limit = %d, want the parent slice's 1 GiB when the process's own cgroup is unlimited", limit)
	}

	host["sys/fs/cgroup/system.slice/ella.service/memory.max"] = &fstest.MapFile{Data: []byte("536870912\n")}

	if limit, _ := MemoryCheck(host, 0); limit != 512<<20 {
		t.Errorf("limit = %d, want the tighter of the service and slice limits", limit)
	}

	host["proc/self/cgroup"] = &fstest.MapFile{Data: []byte("0::/\n")}
	host["sys/fs/cgroup/memory.max"] = &fstest.MapFile{Data: []byte("2147483648\n")}

	if limit, _ := MemoryCheck(host, 0); limit != 2<<30 {
		t.Errorf("limit = %d, want the namespaced cgroup root's 2 GiB", limit)
	}

	if limit, warn := MemoryCheck(fstest.MapFS{}, 1<<40); limit != 0 || warn {
		t.Errorf("missing sources gave limit %d warn %v", limit, warn)
	}
}

func TestWrapCreateError(t *testing.T) {
	s := NewMapSizing(200000)

	err := s.wrapCreateError(unix.ENOMEM, 3<<30)
	if !errors.Is(err, unix.ENOMEM) || !strings.Contains(err.Error(), "raise the memory limit") || !strings.Contains(err.Error(), "3072 MiB") {
		t.Errorf("ENOMEM wrapped as %q", err)
	}

	other := errors.New("verifier")
	if got := s.wrapCreateError(other, 0); got != other {
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
