// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"strings"

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

	var firstErr error

	for _, p := range pkgs {
		// References to MarshalPER/UnmarshalPER methods that this run is
		// about to generate type-check as "undefined" on a first generation;
		// tolerate exactly those.
		kept := p.Errors[:0]

		for _, e := range p.Errors {
			if strings.Contains(e.Msg, "MarshalPER undefined") || strings.Contains(e.Msg, "UnmarshalPER undefined") ||
				strings.Contains(e.Msg, "missing method MarshalPER") || strings.Contains(e.Msg, "missing method UnmarshalPER") {
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

	return pkgs, nil
}
