// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "pergen: %v\n", err)
		os.Exit(1)
	}
}

// run is the actual entry point, separated for testability.
func run(args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}

	pkgs, err := loadPackages(cfg.patterns)
	if err != nil {
		return err
	}

	g := newGenerator(cfg)
	if err := g.generate(pkgs); err != nil {
		return err
	}

	return g.write()
}
