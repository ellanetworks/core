// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"

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
		for _, e := range p.Errors {
			if firstErr == nil {
				firstErr = e
			}
		}
	}

	if firstErr != nil {
		return pkgs, fmt.Errorf("package errors: %w", firstErr)
	}

	return pkgs, nil
}
