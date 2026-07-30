// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package upf

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"testing"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

const tcxTestDev = "elltcx0"

func requireRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() != 0 {
		const msg = "TCX attach requires root/CAP_BPF"
		if os.Getenv("EBPF_REQUIRE_PRIVILEGED") != "" {
			t.Fatal(msg)
		}

		t.Skip(msg + "; skipping")
	}

	if err := rlimit.RemoveMemlock(); err != nil {
		t.Fatalf("cannot remove memlock rlimit: %v", err)
	}
}

func tcxProgramCount(t *testing.T, ifindex int) int {
	t.Helper()

	res, err := link.QueryPrograms(link.QueryOptions{
		Target: ifindex,
		Attach: cebpf.AttachTCXIngress,
	})
	if err != nil {
		t.Fatalf("query TCX programs: %v", err)
	}

	return len(res.Programs)
}

// TestDatapathObjectMatchesAttachMode pins the invariant an explicit
// attach-mode broke: the loaded object has to carry the program type the hook
// accepts, or the attach fails with EINVAL at startup.
func TestDatapathObjectMatchesAttachMode(t *testing.T) {
	requireRoot(t)

	cases := []struct {
		mode string
		want cebpf.ProgramType
	}{
		{config.DatapathTCX, cebpf.SchedCLS},
		{config.DatapathXDPNative, cebpf.XDP},
		{config.DatapathXDPGeneric, cebpf.XDP},
		// The chain starts at native XDP and reloads only if it falls back.
		{config.DatapathChain, cebpf.XDP},
	}

	for _, tc := range cases {
		name := tc.mode
		if name == config.DatapathChain {
			name = "default-chain"
		}

		t.Run(name, func(t *testing.T) {
			objs := ebpf.NewBpfObjects(false, false, 1, 1, 0, 0)
			if err := loadDatapathObjects(objs, tc.mode); err != nil {
				t.Fatalf("load datapath objects: %v", err)
			}

			t.Cleanup(func() { _ = objs.Close() })

			if got := objs.UpfEntryFunc.Type(); got != tc.want {
				t.Errorf("program type = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTCXAttachPinAdopt covers the TCX attach lifecycle: attach pins the
// link, a second attach adopts the pin via link.Update without stacking a
// program on the hook, and a clean Close unpins and detaches.
func TestTCXAttachPinAdopt(t *testing.T) {
	requireRoot(t)

	if out, err := exec.CommandContext(context.Background(), "ip", "link", "add", tcxTestDev, "type", "veth",
		"peer", "name", tcxTestDev+"p").CombinedOutput(); err != nil {
		t.Fatalf("create veth: %v: %s", err, out)
	}

	t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "ip", "link", "del", tcxTestDev).Run() })

	iface, err := net.InterfaceByName(tcxTestDev)
	if err != nil {
		t.Fatalf("lookup %s: %v", tcxTestDev, err)
	}

	obj := ebpf.NewBpfObjects(false, false, iface.Index, iface.Index, 0, 0)
	obj.UseTCX = true

	if err := obj.Load(); err != nil {
		t.Fatalf("load TC objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	l, err := attachTCX(obj.UpfEntryFunc, iface.Index, tcxTestDev)
	if err != nil {
		if tcxUnavailable(err) {
			t.Skipf("kernel without TCX: %v", err)
		}

		t.Fatalf("attach TCX: %v", err)
	}

	pinned, ok := l.(*pinnedLink)
	if !ok {
		t.Fatalf("TCX link is not pinned (bpffs unavailable?)")
	}

	if _, err := os.Stat(pinned.pinPath); err != nil {
		t.Fatalf("pin missing at %s: %v", pinned.pinPath, err)
	}

	if got := tcxProgramCount(t, iface.Index); got != 1 {
		t.Fatalf("TCX programs on hook = %d, want 1", got)
	}

	// A process that died without Close leaves the pin with no open fd; the
	// next attach must adopt it and keep exactly one program on the hook.
	if err := pinned.Link.Close(); err != nil {
		t.Fatalf("close crashed-process fd: %v", err)
	}

	adopted, err := attachTCX(obj.UpfEntryFunc, iface.Index, tcxTestDev)
	if err != nil {
		t.Fatalf("re-attach (adopt) TCX: %v", err)
	}

	if got := tcxProgramCount(t, iface.Index); got != 1 {
		t.Fatalf("TCX programs on hook after adopt = %d, want 1 (hook stacked)", got)
	}

	if err := adopted.Close(); err != nil {
		t.Fatalf("close adopted link: %v", err)
	}

	if _, err := os.Stat(pinned.pinPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pin still present after Close: %v", err)
	}

	if got := tcxProgramCount(t, iface.Index); got != 0 {
		t.Fatalf("TCX programs on hook after Close = %d, want 0", got)
	}

	// The GRO probe backing the TCX attach warning must read this interface.
	if _, err := interfaceGROEnabled(tcxTestDev); err != nil {
		t.Fatalf("read GRO state: %v", err)
	}

	warnGROOnTCX(tcxTestDev)
}
