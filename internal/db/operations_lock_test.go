// Copyright 2026 Ella Networks

package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const operationsLockFile = "operations.lock.json"

// updateLockEnvVar, when "1", regenerates the lock file instead of
// diffing. Run after intentional registry changes:
//
//	ELLA_UPDATE_OPS_LOCK=1 go test ./internal/db/... -run TestOperationsRegistry_AppendOnly
const updateLockEnvVar = "ELLA_UPDATE_OPS_LOCK"

type lockedOp struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	MinSchema int    `json:"minSchema"`
	CmdType   string `json:"cmdType,omitempty"`
}

type lockFile struct {
	Comment    string     `json:"_comment"`
	Operations []lockedOp `json:"operations"`
}

// TestOperationsRegistry_AppendOnly enforces the registry's
// append-only contract: renames, deletes, or relaxed RequireSchema
// fail. New ops fail until the lock file is regenerated.
func TestOperationsRegistry_AppendOnly(t *testing.T) {
	live := buildLockedOps()

	if os.Getenv(updateLockEnvVar) == "1" {
		writeLockFile(t, live)
		t.Logf("regenerated %s", operationsLockFile)

		return
	}

	locked := readLockFile(t)

	lockedByName := make(map[string]lockedOp, len(locked.Operations))
	for _, op := range locked.Operations {
		lockedByName[op.Name] = op
	}

	liveByName := make(map[string]lockedOp, len(live))
	for _, op := range live {
		liveByName[op.Name] = op
	}

	for _, lockedEntry := range locked.Operations {
		liveEntry, present := liveByName[lockedEntry.Name]
		if !present {
			t.Errorf("op %q is in the lock file but not in the live registry — "+
				"renaming or deleting an op breaks rolling upgrades. If you intended this, "+
				"the change must accompany a coordinated cluster-wide upgrade procedure",
				lockedEntry.Name)

			continue
		}

		if liveEntry.Kind != lockedEntry.Kind {
			t.Errorf("op %q changed kind from %q (locked) to %q (live) — "+
				"this changes the wire dispatch path",
				lockedEntry.Name, lockedEntry.Kind, liveEntry.Kind)
		}

		if liveEntry.MinSchema < lockedEntry.MinSchema {
			t.Errorf("op %q minSchema lowered from %d (locked) to %d (live) — "+
				"older nodes that only have schema %d will start applying ops they "+
				"cannot safely run",
				lockedEntry.Name, lockedEntry.MinSchema, liveEntry.MinSchema, lockedEntry.MinSchema)
		}
	}

	for _, liveEntry := range live {
		if _, ok := lockedByName[liveEntry.Name]; !ok {
			t.Errorf("op %q is registered in code but missing from %s — "+
				"add it to the lock file by running: %s=1 go test -run %s ./internal/db/...",
				liveEntry.Name, operationsLockFile, updateLockEnvVar, t.Name())
		}
	}
}

func buildLockedOps() []lockedOp {
	registered := allRegisteredOps()

	out := make([]lockedOp, 0, len(registered))
	for _, op := range registered {
		out = append(out, lockedOp(op))
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out
}

func readLockFile(t *testing.T) lockFile {
	t.Helper()

	path := filepath.Join(".", operationsLockFile)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v\n\nIf this is the first run, generate the file with: %s=1 go test -run %s ./internal/db/...",
			path, err, updateLockEnvVar, t.Name())
	}

	var lf lockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	return lf
}

func writeLockFile(t *testing.T, ops []lockedOp) {
	t.Helper()

	lf := lockFile{
		Comment: "Append-only operation registry. " +
			"Renaming, deleting, or lowering RequireSchema breaks rolling upgrades. " +
			"See spec_rolling_upgrade.md and operations_register.go for the contract. " +
			"Regenerate with ELLA_UPDATE_OPS_LOCK=1 go test -run TestOperationsRegistry_AppendOnly ./internal/db/...",
		Operations: ops,
	}

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		t.Fatalf("marshal lock file: %v", err)
	}

	data = append(data, '\n')

	path := filepath.Join(".", operationsLockFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	fmt.Printf("wrote %s with %d ops\n", path, len(ops))
}
