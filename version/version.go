// Copyright 2024 Ella Networks

package version

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
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

// ProtocolVersion returns the minor component of the embedded semver VERSION
// string as a monotonic integer (e.g. 9 from "v1.9.1"). Any release that adds
// a CommandType, changes a command payload shape, or alters the snapshot format
// must bump the minor version.
func ProtocolVersion() int {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")

	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		panic(fmt.Sprintf("version: cannot parse minor from %q", version))
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(fmt.Sprintf("version: cannot parse minor from %q: %v", version, err))
	}

	return minor
}
