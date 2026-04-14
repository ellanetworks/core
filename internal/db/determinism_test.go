// Copyright 2026 Ella Networks

package db_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestIntentApplyFunctions_AreDeterministic walks the explicit non-changeset
// apply handlers in the db package AST and fails if any of them references a
// source of non-determinism. Each applyX runs once per node against the committed Raft
// log, so two followers replaying the same log must produce byte-identical
// shared.db state. Reading the wall clock, drawing random bytes, generating
// UUIDs, or letting SQLite evaluate CURRENT_TIMESTAMP / RANDOM() inside an
// apply function will silently desync the cluster.
//
// Any non-deterministic value must be captured at propose time (by the
// leader, before Command.MarshalBinary) and carried into the applyX via its
// payload — see applyInsertAuditLog's Timestamp field for an example.
func TestIntentApplyFunctions_AreDeterministic(t *testing.T) {
	// Package-qualified identifiers banned inside applyX bodies. Each entry
	// is matched as a selector (pkg.Name) so unrelated uses like
	// time.Duration are not flagged — only calls to the specific function
	// or package.
	bannedSelectors := map[string]map[string]string{
		"time": {
			"Now":   "captures wall-clock time (differs per node)",
			"Since": "calls time.Now internally",
			"Until": "calls time.Now internally",
			"After": "captures wall-clock time",
			"Tick":  "captures wall-clock time",
		},
		"rand": {
			// Any selector from math/rand or crypto/rand.
			"*": "non-deterministic randomness",
		},
		"uuid": {
			"*": "generates non-deterministic UUIDs",
		},
	}

	// SQL patterns that evaluate non-deterministically on each node when
	// executed as part of a statement body. Matched case-insensitively
	// against every string literal found in an applyX body.
	bannedSQL := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bRANDOM\s*\(`),
		regexp.MustCompile(`(?i)\bCURRENT_TIMESTAMP\b`),
		regexp.MustCompile(`(?i)\bCURRENT_DATE\b`),
		regexp.MustCompile(`(?i)\bCURRENT_TIME\b`),
		regexp.MustCompile(`(?i)\bDATETIME\s*\(\s*'NOW'`),
		regexp.MustCompile(`(?i)\bDATE\s*\(\s*'NOW'`),
		regexp.MustCompile(`(?i)\bTIME\s*\(\s*'NOW'`),
	}

	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	targetedApplyMethods := map[string]struct{}{
		"applyDeleteOldAuditLogs":     {},
		"applyDeleteOldDailyUsage":    {},
		"applyDeleteExpiredSessions":  {},
		"applyDeleteAllDynamicLeases": {},
		"applyMigrateShared":          {},
		"applyRestore":                {},
	}

	var scanned int

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(".", entry.Name())

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			if !isTargetedApplyMethod(fn, targetedApplyMethods) {
				continue
			}

			scanned++

			pos := fset.Position(fn.Pos())

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.SelectorExpr:
					ident, ok := node.X.(*ast.Ident)
					if !ok {
						return true
					}

					rules, banned := bannedSelectors[ident.Name]
					if !banned {
						return true
					}

					reason, specific := rules[node.Sel.Name]
					if !specific {
						reason = rules["*"]
					}

					if reason == "" {
						return true
					}

					t.Errorf("%s: %s uses %s.%s — %s (capture at propose time and pass via payload)",
						pos, fn.Name.Name, ident.Name, node.Sel.Name, reason)

				case *ast.BasicLit:
					if node.Kind != token.STRING {
						return true
					}

					for _, re := range bannedSQL {
						if re.MatchString(node.Value) {
							t.Errorf("%s: %s SQL literal contains non-deterministic expression matching %q: %s",
								pos, fn.Name.Name, re.String(), node.Value)
						}
					}
				}

				return true
			})
		}
	}

	if scanned == 0 {
		t.Fatal("determinism test scanned 0 targeted apply methods — test harness broken")
	}

	t.Logf("scanned %d targeted apply methods", scanned)
}

// isTargetedApplyMethod reports whether fn is one of the explicit non-changeset
// apply handlers still replayed directly from the Raft log.
func isTargetedApplyMethod(fn *ast.FuncDecl, targets map[string]struct{}) bool {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}

	_, ok := targets[fn.Name.Name]

	return ok
}
