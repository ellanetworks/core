// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ci_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// goModules lists every Go module in the repository. The fuzz workflow runs
// `go test` with `working-directory` set to one of these, so a target's
// module is part of its identity.
var goModules = []string{".", "lppa", "nas", "ngap", "nrppa", "per", "s1ap"}

type fuzzTarget struct {
	Dir    string `yaml:"dir"`
	Pkg    string `yaml:"pkg"`
	Target string `yaml:"target"`
}

func (f fuzzTarget) String() string {
	return f.Dir + " " + f.Pkg + " " + f.Target
}

type fuzzWorkflow struct {
	Jobs struct {
		Fuzz struct {
			Strategy struct {
				Matrix struct {
					Include []fuzzTarget `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"fuzz"`
	} `yaml:"jobs"`
}

var fuzzFuncRe = regexp.MustCompile(`(?m)^func (Fuzz[A-Za-z0-9_]*)\(`)

// TestFuzzWorkflowCoversEveryTarget fails when a fuzz target exists in the
// repository but is not scheduled by the nightly workflow, or vice versa.
//
// `go test -fuzz` exits 0 with "no fuzz tests to fuzz" when its regex matches
// nothing, so an unscheduled target is invisible: the job stays green while
// fuzzing nothing. Three of seven matrix entries were in that state — two
// naming targets that did not exist, one matching two targets at once — which
// is why this check exists rather than a comment asking people to remember.
func TestFuzzWorkflowCoversEveryTarget(t *testing.T) {
	root := repoRoot(t)

	declared := parseWorkflowTargets(t, filepath.Join(root, ".github", "workflows", "go-fuzz.yaml"))
	found := scanFuzzTargets(t, root)

	if missing := difference(found, declared); len(missing) > 0 {
		t.Errorf("fuzz targets exist but are never fuzzed; add them to .github/workflows/go-fuzz.yaml:\n  %s",
			strings.Join(missing, "\n  "))
	}

	if stale := difference(declared, found); len(stale) > 0 {
		t.Errorf("go-fuzz.yaml schedules targets that do not exist; the job would report green while fuzzing nothing:\n  %s",
			strings.Join(stale, "\n  "))
	}
}

// TestFuzzWorkflowRegexesAreAnchored guards the other half of the failure:
// an unanchored name that matches two targets makes the job fail nightly,
// and one that matches a prefix of another silently fuzzes the wrong thing.
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

func parseWorkflowTargets(t *testing.T, path string) []string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var wf fuzzWorkflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	include := wf.Jobs.Fuzz.Strategy.Matrix.Include
	if len(include) == 0 {
		t.Fatalf("%s declares no fuzz targets", path)
	}

	out := make([]string, 0, len(include))
	for _, e := range include {
		out = append(out, e.String())
	}

	return out
}

// scanFuzzTargets walks each module for `func Fuzz*` in _test.go files and
// returns them in the workflow's dir/pkg/target form.
func scanFuzzTargets(t *testing.T, root string) []string {
	t.Helper()

	nested := make(map[string]bool, len(goModules))

	for _, m := range goModules {
		if m != "." {
			nested[m] = true
		}
	}

	var out []string

	for _, mod := range goModules {
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

				// A nested module is walked under its own entry, not the root's.
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
