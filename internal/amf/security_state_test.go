// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAuthProofMintSites enforces that the two AuthProof constructors
// are called only from the authorized files. This is the compile-time
// equivalent of "you cannot mint an AuthProof outside these files" —
// because Go does not support package-private symbol exports, the
// guarantee is instead pinned by this grep test.
//
// Any change to this list is a security-boundary change and should be
// reviewed accordingly.
func TestAuthProofMintSites(t *testing.T) {
	allowed := map[string]map[string]struct{}{
		"MintAuthProofForSMC": {
			"internal/amf/nas/gmm/handle_security_mode_complete.go": {},
		},
		"MintAuthProofForInitialRegistration": {
			"internal/amf/nas/gmm/handle_registration_request.go": {},
		},
	}

	// Walk from the repository root so the allowed paths match.
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

		// Skip the declaration site and this test itself.
		rel, _ := filepath.Rel(root, path)
		if rel == "internal/amf/security_state.go" || rel == "internal/amf/security_state_test.go" {
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

// repoRoot locates the repository root by walking upwards from this
// test file's directory until a go.mod is found.
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
