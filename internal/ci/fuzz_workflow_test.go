// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ci_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type fuzzTarget struct {
	Dir    string `json:"dir"`
	Pkg    string `json:"pkg"`
	Target string `json:"target"`
}

func (f fuzzTarget) String() string {
	return f.Dir + " " + f.Pkg + " " + f.Target
}

var fuzzFuncRe = regexp.MustCompile(`(?m)^func (Fuzz[A-Za-z0-9_]*)\(\s*[A-Za-z_][A-Za-z0-9_]*\s+\*testing\.F\s*\)`)

func TestFuzzDiscoveryFindsEveryTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: the discovery script links a test binary for every package in every module")
	}

	root := repoRoot(t)

	discovered := runDiscoveryScript(t, root)
	found := scanFuzzTargets(t, root, goModules(t, root))

	if missing := difference(found, discovered); len(missing) > 0 {
		t.Errorf("fuzz targets exist but discovery does not emit them, so they are never fuzzed:\n  %s",
			strings.Join(missing, "\n  "))
	}

	if stale := difference(discovered, found); len(stale) > 0 {
		t.Errorf("discovery emits targets that do not exist; the job would report green while fuzzing nothing:\n  %s",
			strings.Join(stale, "\n  "))
	}
}

func runDiscoveryScript(t *testing.T, root string) []string {
	t.Helper()

	script := filepath.Join(root, ".github", "scripts", "list-fuzz-targets.sh")

	cmd := exec.CommandContext(t.Context(), "bash", script)
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run %s: %v", script, err)
	}

	var entries []fuzzTarget
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("decode discovery output: %v (output=%q)", err, out)
	}

	if len(entries) == 0 {
		t.Fatal("discovery emitted no targets")
	}

	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.String())
	}

	return ids
}

func TestFuzzWorkflowRegexesAreAnchored(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "go-fuzz.yaml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}

	if !strings.Contains(string(raw), `-fuzz='^${{ matrix.target }}$'`) {
		t.Error("the -fuzz argument must be anchored as '^${{ matrix.target }}$'; " +
			"an unanchored name matching two targets fails the job, and one matching none passes silently")
	}
}

func goModules(t *testing.T, root string) []string {
	t.Helper()

	var mods []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}

			return nil
		}

		if d.Name() != "go.mod" {
			return nil
		}

		rel, relErr := filepath.Rel(root, filepath.Dir(path))
		if relErr != nil {
			return relErr
		}

		mods = append(mods, filepath.ToSlash(rel))

		return nil
	})
	if err != nil {
		t.Fatalf("walk for go.mod: %v", err)
	}

	if len(mods) == 0 {
		t.Fatal("found no go.mod in the repository; the module scanner is broken")
	}

	sort.Strings(mods)

	return mods
}

func scanFuzzTargets(t *testing.T, root string, modules []string) []string {
	t.Helper()

	nested := make(map[string]bool, len(modules))

	for _, m := range modules {
		if m != "." {
			nested[m] = true
		}
	}

	var out []string

	for _, mod := range modules {
		modRoot := filepath.Join(root, mod)

		err := filepath.WalkDir(modRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			rel, relErr := filepath.Rel(modRoot, path)
			if relErr != nil {
				return relErr
			}

			if d.IsDir() {
				base := d.Name()
				if base == ".git" || base == "node_modules" || base == "testdata" {
					return filepath.SkipDir
				}

				if mod == "." && nested[rel] {
					return filepath.SkipDir
				}

				return nil
			}

			if !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}

			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}

			pkg := "./" + filepath.ToSlash(filepath.Dir(rel))
			if filepath.Dir(rel) == "." {
				pkg = "."
			}

			for _, m := range fuzzFuncRe.FindAllStringSubmatch(string(src), -1) {
				out = append(out, fuzzTarget{Dir: mod, Pkg: pkg, Target: m[1]}.String())
			}

			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", modRoot, err)
		}
	}

	if len(out) == 0 {
		t.Fatal("found no fuzz targets in the repository; the scanner is broken")
	}

	return out
}

func difference(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, s := range b {
		inB[s] = true
	}

	var out []string

	for _, s := range a {
		if !inB[s] {
			out = append(out, s)
		}
	}

	sort.Strings(out)

	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".github", "workflows")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root")
		}

		dir = parent
	}
}
