// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	writeFile = func(_, filename string, data []byte) error {
		cleaned := filepath.Clean(filename)

		if strings.Contains(cleaned, "..") {
			return fmt.Errorf("output path escapes source directory: %s", filename)
		}

		return os.WriteFile(cleaned, data, 0o600)
	}
}
