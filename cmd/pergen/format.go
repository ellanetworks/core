// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"go/format"
)

func init() {
	goFormatImpl = format.Source
}
