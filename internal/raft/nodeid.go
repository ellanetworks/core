// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// MaxNodeID is the upper bound for node IDs, constrained by the 6-bit
	// AMF Pointer field in 3GPP TS 23.003 §2.10.1.
	MaxNodeID = 63

	// nodeIDFilename is the file within the data directory that persists
	// the node ID across restarts.
	nodeIDFilename = "node-id"
)

// ResolveNodeID determines the node ID using the following precedence chain:
//  1. configNodeID (from YAML cluster.node-id)
//  2. <dataDir>/node-id file (written on first HA boot)
//  3. Error — operator must assign one
//
// Once resolved, the ID is written to <dataDir>/node-id. On subsequent boots,
// the persisted value is validated against config; mismatches fail loudly.
func ResolveNodeID(configNodeID int, dataDir string) (int, error) {
	return resolveNodeID(configNodeID, dataDir, 0)
}

// resolveNodeID implements ResolveNodeID. A non-zero fallback is adopted and
// persisted when no source supplies an ID, instead of returning an error.
func resolveNodeID(configNodeID int, dataDir string, fallback int) (int, error) {
	nodeIDPath := filepath.Join(dataDir, nodeIDFilename)

	resolved, source := resolveFromSources(configNodeID)

	if resolved == 0 {
		// Try the persisted file.
		persisted, err := readNodeIDFile(nodeIDPath)
		if err == nil {
			return persisted, nil
		}

		if fallback == 0 {
			return 0, fmt.Errorf("node ID not provided: set cluster.node-id in config, "+
				"or ensure %s exists: %w", nodeIDPath, err)
		}

		resolved, source = fallback, "standalone default"
	}

	if err := validateNodeID(resolved); err != nil {
		return 0, fmt.Errorf("node ID from %s: %w", source, err)
	}

	// If the file already exists, validate consistency.
	persisted, fileErr := readNodeIDFile(nodeIDPath)
	if fileErr == nil && persisted != resolved {
		return 0, fmt.Errorf("node ID mismatch: %s says %d but %s contains %d",
			source, resolved, nodeIDPath, persisted)
	}

	// Write (or overwrite) the file.
	if err := writeNodeIDFile(nodeIDPath, resolved); err != nil {
		return 0, fmt.Errorf("persist node ID: %w", err)
	}

	return resolved, nil
}

func resolveFromSources(configNodeID int) (int, string) {
	if configNodeID != 0 {
		return configNodeID, "config (cluster.node-id)"
	}

	return 0, ""
}

func validateNodeID(id int) error {
	if id < 1 || id > MaxNodeID {
		return fmt.Errorf("must be between 1 and %d, got %d", MaxNodeID, id)
	}

	return nil
}

func readNodeIDFile(path string) (int, error) {
	data, err := os.ReadFile(path) // #nosec: G304 — path is under our data directory
	if err != nil {
		return 0, err
	}

	id, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid node-id file content %q: %w", string(data), err)
	}

	if err := validateNodeID(id); err != nil {
		return 0, fmt.Errorf("persisted node-id: %w", err)
	}

	return id, nil
}

func writeNodeIDFile(path string, id int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(id)+"\n"), 0o600)
}
