// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import "os"

func init() {
	writeFile = func(filename string, data []byte) error {
		return os.WriteFile(filename, data, 0o600)
	}
}
