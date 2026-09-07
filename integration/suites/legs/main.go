// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ellanetworks/core/integration/suites"
)

const maxMatrixJobs = 256

func main() {
	legs := suites.Legs()

	if len(legs) == 0 {
		fmt.Fprintln(os.Stderr, "no integration legs found; the suite registry is broken")
		os.Exit(1)
	}

	if len(legs) > maxMatrixJobs {
		fmt.Fprintf(os.Stderr, "%d legs exceeds the %d-job GitHub Actions matrix limit\n", len(legs), maxMatrixJobs)
		os.Exit(1)
	}

	out, err := json.Marshal(legs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal legs: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}
