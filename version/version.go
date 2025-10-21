// Copyright 2024 Ella Networks

package version

import (
	_ "embed"
)

//go:embed VERSION
var version string

// GitCommit is the git commit hash which is set during build time via -ldflags
var GitCommit string

type VersionInfo struct {
	Version  string
	Revision string
}

func GetVersion() *VersionInfo {
	return &VersionInfo{
		Revision: GitCommit,
		Version:  version,
	}
}
