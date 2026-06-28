// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAuthProofMintSites enforces that the AuthProof constructors are called
// only from the authorized files — the grep equivalent of "you cannot mint an
// AuthProof outside these files". It does NOT catch in-package AuthProof{}
// struct-literal abuses (the unexported field blocks that outside the mme
// package; inside it relies on reviewer vigilance). Any change to this list is a
// security-boundary change.
func TestAuthProofMintSites(t *testing.T) {
	allowed := map[string]map[string]struct{}{
		"MintAuthProofForSecurityMode": {
			"internal/mme/emm.go": {},
		},
		"MintAuthProofForAttachCommit": {
			"internal/mme/bearer.go": {},
		},
	}

	root, err := repoRoot(t)
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	patterns := map[string]*regexp.Regexp{}
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

		// Tests are trusted to mint an AuthProof to exercise the gated methods;
		// the boundary this enforces is on production code.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "internal/mme/security_state.go" || rel == "internal/mme/security_state_test.go" {
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
				t.Errorf("AuthProof mint function %s called from unauthorized file %s", name, rel)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// repoRoot locates the repository root by walking upwards until a go.mod is found.
func repoRoot(t *testing.T) (string, error) {
	t.Helper()

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
