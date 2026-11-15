// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ellanetworks/core/per/pergen"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "pergen: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}

	out, err := pergen.Generate(context.Background(), cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(cfg.Output, out, 0o600)
}
