// Copyright 2024 Ella Networks

package version

import (
	_ "embed"
)

//go:embed VERSION
var version string

func GetVersion() string {
	return version
}
