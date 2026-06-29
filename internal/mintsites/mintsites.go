// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package mintsites is a test helper that enforces a call-site allowlist: named
// functions may be referenced only from an explicit set of files.
//
// It backs the AuthProof mint-site boundary in the amf and mme packages. Because
// Go has no package-private symbol export, the "this token can only be minted
// here" guarantee cannot be expressed in the type system: a shared AuthProof type
// would need an exported constructor and so be forgeable from any importer. Each
// RAT package therefore keeps its own unexported-field AuthProof and Mint*
// functions, and this grep-based test pins the set of authorized call sites.
package mintsites

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Check fails t if any *.go file outside the allowlist references one of the
// named functions. allowed maps each function name to the set of repo-relative
// paths permitted to reference it. skipFiles are repo-relative paths excluded
// from the scan (the declaration site and this boundary's own test). When
// skipTests is true, *_test.go files are trusted and skipped — they may mint to
// exercise the gated methods.
//
// Any change to an allowed map is a security-boundary change and should be
// reviewed accordingly.
func Check(t *testing.T, allowed map[string]map[string]struct{}, skipFiles []string, skipTests bool) {
	t.Helper()

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	skip := make(map[string]struct{}, len(skipFiles))
	for _, f := range skipFiles {
		skip[f] = struct{}{}
	}

	patterns := make(map[string]*regexp.Regexp, len(allowed))
	for name := range allowed {
		patterns[name] = regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "dist" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if skipTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		if _, ok := skip[rel]; ok {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for name, re := range patterns {
			if !re.Match(content) {
				continue
			}

			if _, ok := allowed[name][rel]; !ok {
				t.Errorf("mint function %s referenced from unauthorized file %s", name, rel)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// repoRoot locates the repository root by walking upwards until a go.mod is found.
func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}

		dir = parent
	}
}
