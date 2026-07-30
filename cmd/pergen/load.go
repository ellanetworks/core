// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"go/types"
	"os"
	"regexp"

	"golang.org/x/tools/go/packages"
)

// loadPackages loads Go packages matching patterns. It uses go/packages
// (which drives the Go type checker), not reflect. We request syntax, types,
// and types info so we can inspect structs and named types.
func loadPackages(patterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched %v", patterns)
	}

	var (
		firstErr  error
		tolerated int
	)

	for _, p := range pkgs {
		kept := p.Errors[:0]

		for _, e := range p.Errors {
			if pendingMethodError(p, e) {
				tolerated++
				continue
			}

			kept = append(kept, e)

			if firstErr == nil {
				firstErr = e
			}
		}

		p.Errors = kept
	}

	if firstErr != nil {
		return pkgs, fmt.Errorf("package errors: %w", firstErr)
	}
	// Tolerated errors are expected on a first generation and when the
	// committed output is stale, but never on a steady-state regeneration —
	// so report them rather than swallowing them silently.
	if tolerated > 0 {
		fmt.Fprintf(os.Stderr, "pergen: %d reference(s) to codec methods this run will generate\n", tolerated)
	}

	return pkgs, nil
}

// perMethodErrPatterns match the two type-checker diagnostics emitted when
// source references a MarshalPER/UnmarshalPER method that does not exist yet.
// Each captures the named type the method is missing from.
var perMethodErrPatterns = []*regexp.Regexp{
	regexp.MustCompile(`type \*?(\w+) has no field or method (?:Marshal|Unmarshal)PER`),
	regexp.MustCompile(`\*?(\w+) does not implement per\.(?:Marshaler|Unmarshaler) \(missing method (?:Marshal|Unmarshal)PER\)`),
}

// pendingMethodError reports whether e is a reference to a codec method on a
// type declared in this package — one this run is about to generate. Matching
// on the message alone would swallow genuine type errors that happen to
// mention those method names, so the named type must also resolve locally.
func pendingMethodError(pkg *packages.Package, e packages.Error) bool {
	if pkg.Types == nil {
		return false
	}

	for _, re := range perMethodErrPatterns {
		m := re.FindStringSubmatch(e.Msg)
		if m == nil {
			continue
		}

		if obj, ok := pkg.Types.Scope().Lookup(m[1]).(*types.TypeName); ok && obj != nil {
			return true
		}
	}

	return false
}
