// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package ui

import (
	"embed"
)

//go:embed all:frontend_files
var FrontendFS embed.FS
