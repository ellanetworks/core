package version

import (
	_ "embed"
)

//go:embed VERSION
var version string

func GetVersion() string {
	return version
}
