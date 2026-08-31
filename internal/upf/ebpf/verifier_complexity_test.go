// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

const verifierInstructionBudget = 1_000_000

const (
	verifierBudgetFail = 0.7
	verifierBudgetWarn = 0.5
)

var processedInsns = regexp.MustCompile(`processed (\d+) insns`)

type datapathConfig struct {
	name       string
	masquerade bool
	flowact    bool
}

var datapathConfigs = []datapathConfig{
	{"plain", false, false},
	{"masquerade", true, false},
	{"flowact", false, true},
	{"masquerade+flowact", true, true},
}

func TestVerifierComplexityHeadroom(t *testing.T) {
	requireProgTestRun(t)

	builds := map[string]func() (*ebpf.CollectionSpec, error){
		"xdp": LoadN3N6Entrypoint,
		"tc":  LoadN3N6EntrypointTc,
	}

	for build, load := range builds {
		t.Run(build, func(t *testing.T) {
			for _, cfg := range datapathConfigs {
				t.Run(cfg.name, func(t *testing.T) {
					spec, err := load()
					if err != nil {
						t.Fatalf("load %s collection spec: %v", build, err)
					}

					for name := range spec.Programs {
						t.Run(name, func(t *testing.T) {
							insns := loadOneProgram(t, spec, name, cfg)
							share := float64(insns) * 100 / verifierInstructionBudget

							t.Logf("%s/%s: %d instructions (%.1f%% of budget) on %s",
								cfg.name, name, insns, share, kernelRelease())

							checkBudget(t, name, cfg.name, insns)
						})
					}
				})
			}
		})
	}
}

func checkBudget(t *testing.T, prog, cfg string, insns int) {
	t.Helper()

	fail := int(verifierBudgetFail * verifierInstructionBudget)
	warn := int(verifierBudgetWarn * verifierInstructionBudget)

	switch {
	case insns > fail:
		msg := "%s spends %d verifier instructions under %s on %s, over the %d error threshold (%.0f%% of the %d budget): the datapath stops loading at %d"
		args := []any{
			prog, insns, cfg, kernelRelease(), fail,
			verifierBudgetFail * 100, verifierInstructionBudget,
			verifierInstructionBudget,
		}

		if os.Getenv("EBPF_REQUIRE_PRIVILEGED") == "" {
			t.Logf("(not enforced off CI) "+msg, args...)

			return
		}

		t.Errorf(msg, args...)
	case insns > warn:
		t.Logf("%s spends %d verifier instructions under %s on %s, over the %d warning threshold",
			prog, insns, cfg, kernelRelease(), warn)
	}
}

func kernelRelease() string {
	var u unix.Utsname

	if err := unix.Uname(&u); err != nil {
		return "unknown"
	}

	return unix.ByteSliceToString(u.Release[:])
}

func loadOneProgram(t *testing.T, spec *ebpf.CollectionSpec, name string, cfg datapathConfig) int {
	t.Helper()

	one := &ebpf.CollectionSpec{
		Maps:      spec.Maps,
		Programs:  map[string]*ebpf.ProgramSpec{name: spec.Programs[name]},
		Variables: spec.Variables,
		Types:     spec.Types,
		ByteOrder: spec.ByteOrder,
	}

	vars := map[string]any{
		"masquerade":   cfg.masquerade,
		"flowact":      cfg.flowact,
		"n3_ifindex":   uint32(2),
		"n6_ifindex":   uint32(3),
		"nat_port_min": NatPortMin,
		"nat_port_max": NatPortMax,
	}

	for k, v := range vars {
		vr, ok := one.Variables[k]
		if !ok {
			continue
		}

		if err := vr.Set(v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	coll, err := ebpf.NewCollectionWithOptions(one, ebpf.CollectionOptions{
		Programs: ebpf.ProgramOptions{LogLevel: ebpf.LogLevelStats},
	})
	if err != nil {
		if _, verbose := ebpf.NewCollectionWithOptions(one, ebpf.CollectionOptions{
			Programs: ebpf.ProgramOptions{LogLevel: ebpf.LogLevelInstruction},
		}); verbose != nil {
			t.Fatalf("load %s: %+v", name, verbose)
		}

		t.Fatalf("load %s: %v", name, err)
	}

	defer coll.Close()

	log := coll.Programs[name].VerifierLog

	m := processedInsns.FindStringSubmatch(log)
	if m == nil {
		t.Skipf("kernel reported no instruction count for %s; verifier log: %q", name, log)
	}

	insns, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("parse instruction count %q: %v", m[1], err)
	}

	return insns
}
