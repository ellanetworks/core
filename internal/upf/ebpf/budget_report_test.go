// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
)

var (
	reProcessed = regexp.MustCompile(`processed (\d+) insns`)
	reStates    = regexp.MustCompile(`total_states (\d+)`)
	rePeak      = regexp.MustCompile(`peak_states (\d+)`)
	reMaxPer    = regexp.MustCompile(`max_states_per_insn (\d+)`)
	reSrcLine   = regexp.MustCompile(`@ ([^ ]+\.[ch]):(\d+)`)
	reInsn      = regexp.MustCompile(`^\d+: \(`)
)

func num(re *regexp.Regexp, s string) int {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return -1
	}

	n, _ := strconv.Atoi(m[1])

	return n
}

func applyCfg(t *testing.T, spec *ebpf.CollectionSpec, cfg datapathConfig) {
	t.Helper()

	vals := map[string]any{
		"masquerade": cfg.masquerade, "flowact": cfg.flowact,
		"n3_ifindex": uint32(2), "n6_ifindex": uint32(3),
		"nat_port_min": NatPortMin, "nat_port_max": NatPortMax,
	}

	for k, v := range vals {
		if vr, ok := spec.Variables[k]; ok {
			if err := vr.Set(v); err != nil {
				t.Fatalf("set %s: %v", k, err)
			}
		}
	}
}

func TestBudgetReport(t *testing.T) {
	requireProgTestRun(t)

	fmt.Printf("KERNEL\t%s\n", kernelRelease())

	for _, b := range []struct {
		name string
		load func() (*ebpf.CollectionSpec, error)
	}{{"xdp", LoadN3N6Entrypoint}, {"tc", LoadN3N6EntrypointTc}} {
		spec, err := b.load()
		if err != nil {
			t.Fatal(err)
		}

		if b.name == "xdp" {
			names := make([]string, 0, len(spec.Maps))
			for n := range spec.Maps {
				names = append(names, n)
			}

			sort.Strings(names)

			for _, n := range names {
				m := spec.Maps[n]
				fmt.Printf("MAP\t%s\t%s\t%d\t%d\t%d\t%d\n", n, m.Type,
					m.KeySize, m.ValueSize, m.MaxEntries,
					(int(m.KeySize)+int(m.ValueSize))*int(m.MaxEntries))
			}
		}

		progNames := make([]string, 0, len(spec.Programs))
		for n := range spec.Programs {
			progNames = append(progNames, n)
		}

		sort.Strings(progNames)

		for _, cfg := range datapathConfigs {
			for _, name := range progNames {
				one, err := b.load()
				if err != nil {
					t.Fatal(err)
				}

				ins := one.Programs[name].Instructions
				frames := functionFrames(ins)
				stack := deepestChain(name, frames, functionCalls(ins), map[string]bool{})

				used := map[string]bool{}

				for _, i := range ins {
					if i.IsLoadFromMap() && i.Reference() != "" {
						used[i.Reference()] = true
					}
				}

				for other := range one.Programs {
					if other != name {
						delete(one.Programs, other)
					}
				}

				applyCfg(t, one, cfg)

				coll, err := ebpf.NewCollectionWithOptions(one, ebpf.CollectionOptions{
					Programs: ebpf.ProgramOptions{LogLevel: ebpf.LogLevelStats},
				})
				if err != nil {
					fmt.Printf("PROG\t%s\t%s\t%s\tLOADFAIL\n", b.name, cfg.name, name)

					continue
				}

				log := coll.Programs[name].VerifierLog

				coll.Close()

				fmt.Printf("PROG\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
					b.name, cfg.name, name, num(reProcessed, log),
					num(reStates, log), num(rePeak, log), num(reMaxPer, log),
					stack, len(ins), len(used), len(frames))
			}
		}
	}
}

func TestBudgetAttribution(t *testing.T) {
	requireProgTestRun(t)

	const name = "upf_uplink_func"

	spec, err := LoadN3N6EntrypointTc()
	if err != nil {
		t.Fatal(err)
	}

	for other := range spec.Programs {
		if other != name {
			delete(spec.Programs, other)
		}
	}

	applyCfg(t, spec, datapathConfig{"worst", true, true})

	coll, err := ebpf.NewCollectionWithOptions(spec, ebpf.CollectionOptions{
		Programs: ebpf.ProgramOptions{
			LogLevel:     ebpf.LogLevelInstruction | ebpf.LogLevelStats,
			LogSizeStart: 512 * 1024 * 1024,
		},
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	log := coll.Programs[name].VerifierLog

	coll.Close()

	perFile := map[string]int{}
	perLine := map[string]int{}
	cur := "(none)"
	total := 0

	for line := range strings.SplitSeq(log, "\n") {
		if m := reSrcLine.FindStringSubmatch(line); m != nil {
			cur = m[1] + ":" + m[2]

			continue
		}

		if !reInsn.MatchString(line) {
			continue
		}

		perFile[strings.SplitN(cur, ":", 2)[0]]++
		perLine[cur]++
		total++
	}

	fmt.Printf("ATTRTOTAL\t%d\n", total)

	type kv struct {
		k string
		v int
	}

	rows := make([]kv, 0, len(perFile))
	for k, v := range perFile {
		rows = append(rows, kv{k, v})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].v > rows[j].v })

	for _, r := range rows {
		fmt.Printf("ATTRFILE\t%s\t%d\t%.1f\n", r.k, r.v, float64(r.v)*100/float64(total))
	}

	lines := make([]kv, 0, len(perLine))
	for k, v := range perLine {
		lines = append(lines, kv{k, v})
	}

	sort.Slice(lines, func(i, j int) bool { return lines[i].v > lines[j].v })

	for i, r := range lines {
		if i >= 12 {
			break
		}

		fmt.Printf("ATTRLINE\t%s\t%d\t%.1f\n", r.k, r.v, float64(r.v)*100/float64(total))
	}
}

func TestStackSlotAttribution(t *testing.T) {
	spec, err := LoadN3N6EntrypointTc()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"upf_uplink_func", "upf_downlink_func"} {
		owners := map[int]map[string]bool{}
		maxOff := 0
		inMain := true

		for _, ins := range spec.Programs[name].Instructions {
			if sym := ins.Symbol(); sym != "" {
				inMain = sym == name
			}

			if !inMain {
				continue
			}

			switch ins.OpCode.Class() {
			case asm.LdXClass, asm.StXClass, asm.StClass:
			default:
				continue
			}

			if ins.Dst != asm.R10 && ins.Src != asm.R10 {
				continue
			}

			off := -int(ins.Offset)
			if off <= 0 {
				continue
			}

			if off > maxOff {
				maxOff = off
			}

			slot := (off - 1) / 8

			file := "(no line info)"
			if src := ins.Source(); src != nil {
				if l, ok := src.(interface{ FileName() string }); ok {
					parts := strings.Split(l.FileName(), "/")
					file = parts[len(parts)-1]
				}
			}

			if owners[slot] == nil {
				owners[slot] = map[string]bool{}
			}

			owners[slot][file] = true
		}

		shared := 0
		excl := map[string]int{}

		for _, fs := range owners {
			if len(fs) == 1 {
				for f := range fs {
					excl[f] += 8
				}

				continue
			}

			shared += 8
		}

		fmt.Printf("SLOTS\t%s\textent=%d\tslots=%d\tshared=%d\texclusive=%v\n",
			name, maxOff, len(owners), shared, excl)
	}
}
