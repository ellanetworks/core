// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ellanetworks/core/internal/pki"
)

const nodeIDFilename = "node-id"

func ResolveRaftID(dataDir string) (string, error) {
	path := filepath.Join(dataDir, nodeIDFilename)

	data, err := os.ReadFile(path) // #nosec: G304 — path is under our data directory
	if err == nil {
		id, nerr := pki.NormalizeNodeID(string(data))
		if nerr != nil {
			return "", fmt.Errorf("read %s: %w", path, nerr)
		}

		return id, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	id, err := pki.NewNodeID()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("persist node id: %w", err)
	}

	return id, nil
}
