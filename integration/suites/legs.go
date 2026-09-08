// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package suites

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const setupHeadroomMinutes = 15

type Leg struct {
	Name           string `json:"name"`
	Suite          string `json:"suite"`
	Timeout        string `json:"timeout"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	NeedsTester    bool   `json:"needs_tester"`
	Setup          string `json:"setup"`
	Run            string `json:"run"`
	Skip           string `json:"skip"`
	Cell
}

func BuildLegs(decls []Declaration, subtreePrefixes []string) ([]Leg, error) {
	bySuite := map[string][]Declaration{}
	for _, d := range decls {
		bySuite[d.Suite] = append(bySuite[d.Suite], d)
	}

	names := make([]string, 0, len(bySuite))
	for n := range bySuite {
		names = append(names, n)
	}

	sort.Strings(names)

	var undeclared []string

	for name := range Definitions {
		if _, ok := bySuite[string(name)]; !ok {
			undeclared = append(undeclared, string(name))
		}
	}

	if len(undeclared) > 0 {
		sort.Strings(undeclared)

		return nil, fmt.Errorf("these suites are defined but no test declares them, so they would run nothing: %s",
			strings.Join(undeclared, ", "))
	}

	var legs []Leg

	for _, name := range names {
		def, ok := Definitions[Name(name)]
		if !ok {
			return nil, fmt.Errorf("suite %q has declarations but no definition", name)
		}

		run, skip, err := patterns(bySuite[name], subtreePrefixes)
		if err != nil {
			return nil, fmt.Errorf("suite %s: %w", name, err)
		}

		minutes, err := strconv.Atoi(strings.TrimSuffix(def.Timeout, "m"))
		if err != nil {
			return nil, fmt.Errorf("suite %s: timeout %q must be whole minutes", name, def.Timeout)
		}

		for _, c := range def.Profile.Cells {
			legs = append(legs, Leg{
				Name:           fmt.Sprintf("%s (%s, %s, %s)", name, c.Arch, c.IPFamily, c.AttachMode),
				Suite:          name,
				Timeout:        def.Timeout,
				TimeoutMinutes: minutes + setupHeadroomMinutes,
				NeedsTester:    def.NeedsTester,
				Setup:          def.Setup,
				Run:            run,
				Skip:           skip,
				Cell:           c,
			})
		}
	}

	return legs, nil
}

func patterns(decls []Declaration, subtreePrefixes []string) (string, string, error) {
	tests := make([]string, 0, len(decls))
	skips := make([]string, 0, 2)

	for _, d := range decls {
		tests = append(tests, d.Test)

		switch {
		case d.SubtestPrefix != "":
			others := complement(subtreePrefixes, d.SubtestPrefix)
			if len(others) == 0 {
				return "", "", fmt.Errorf("%s: subtest prefix %q has no complement to skip", d.Test, d.SubtestPrefix)
			}

			skips = append(skips, fmt.Sprintf("^%s$/^(%s)$", d.Test, strings.Join(others, "|")))
		case d.ExcludeSubtest != "":
			skips = append(skips, fmt.Sprintf("^%s$/^(%s)$", d.Test, d.ExcludeSubtest))
		}
	}

	sort.Strings(tests)
	sort.Strings(skips)

	return fmt.Sprintf("^(%s)$", strings.Join(tests, "|")), strings.Join(skips, "|"), nil
}

func complement(all []string, exclude string) []string {
	out := make([]string, 0, len(all))

	for _, p := range all {
		if p != exclude {
			out = append(out, p)
		}
	}

	sort.Strings(out)

	return out
}
