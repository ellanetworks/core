// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pkiagent

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestHaveLeafOnDiskRejectsMismatchedPair(t *testing.T) {
	a := NewAgent(1, "cluster-a", t.TempDir())
	if err := a.GenerateAndPersist(); err != nil {
		t.Fatalf("generate-and-persist: %v", err)
	}

	if !a.HaveLeafOnDisk() {
		t.Fatal("a freshly persisted keypair must be reported as present")
	}

	other := NewAgent(1, "cluster-a", t.TempDir())
	if err := other.GenerateAndPersist(); err != nil {
		t.Fatalf("generate-and-persist other: %v", err)
	}

	strayKey, err := os.ReadFile(other.path(leafKeyFile))
	if err != nil {
		t.Fatalf("read other key: %v", err)
	}

	if err := os.WriteFile(a.path(leafKeyFile), strayKey, 0o600); err != nil {
		t.Fatalf("write stray key: %v", err)
	}

	if a.HaveLeafOnDisk() {
		t.Fatal("a key that does not pair with the cert must be reported as absent so the caller can rejoin")
	}
}

func TestAtomicWritePreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leaf.key")

	if err := atomicWrite(path, []byte("secret"), 0o600); err != nil {
		t.Fatalf("atomic write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode is %o, want 600", info.Mode().Perm())
	}
}

func TestAtomicWriteConcurrentWritersNeverPublishPartialContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peer-pins.json")

	payloads := make([][]byte, 8)
	for i := range payloads {
		payloads[i] = bytes.Repeat([]byte{byte('a' + i)}, 256<<10)
	}

	var wg sync.WaitGroup

	for _, data := range payloads {
		wg.Go(func() {
			for range 20 {
				if err := atomicWrite(path, data, 0o644); err != nil {
					t.Errorf("atomic write: %v", err)

					return
				}
			}
		})
	}

	wg.Wait()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	for _, data := range payloads {
		if bytes.Equal(got, data) {
			return
		}
	}

	t.Fatalf("published file is not any writer's complete payload (len=%d)", len(got))
}

func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peer-pins.json")

	for range 5 {
		if err := atomicWrite(path, []byte("payload"), 0o644); err != nil {
			t.Fatalf("atomic write: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	if len(entries) != 1 || entries[0].Name() != "peer-pins.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}

		t.Fatalf("staging files left behind: %v", names)
	}
}
