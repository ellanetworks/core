// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var parseFuncRE = regexp.MustCompile(`(?m)^func (Parse[A-Za-z0-9]+)\(`)

// TestEveryParserIsRegistered fails when an exported ParseXxx function is not
// listed in messageParsers. The registry drives the no-panic fuzzer, so an
// unregistered parser is an unfuzzed decode path — exactly the gap that lets a
// panic on hostile input reach production.
func TestEveryParserIsRegistered(t *testing.T) {
	registered := make(map[string]bool, len(messageParsers)+len(transferParsers))

	for _, p := range append(append([]messageParser{}, messageParsers...), transferParsers...) {
		if registered[p.Name] {
			t.Errorf("parser %s registered twice", p.Name)
		}

		registered[p.Name] = true
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	var found int

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}

		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range parseFuncRE.FindAllStringSubmatch(string(src), -1) {
			found++

			if !registered[m[1]] {
				t.Errorf("%s declares %s but it is in neither messageParsers nor transferParsers (add it, or the fuzzer will never reach it)", name, m[1])
			}
		}
	}

	if want := len(messageParsers) + len(transferParsers); found != want {
		t.Errorf("found %d ParseXxx functions in source, registries have %d", found, want)
	}

	if found == 0 {
		t.Fatal("no ParseXxx functions found; the source scan is broken")
	}
}
