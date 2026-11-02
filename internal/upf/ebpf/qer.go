// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"
	"fmt"

	"github.com/cilium/ebpf"
)

const MaxSessionQERs = 1 + MaxClassifierTargets

func (bpfObjects *BpfObjects) CreateQerMeter(seid uint64, id uint32) error {
	key := N3N6EntrypointQerKey{Seid: seid, QerId: id}

	value := make([]byte, bpfObjects.QerWindows.ValueSize())
	if err := bpfObjects.insertTracked(bpfObjects.QerWindows, MapQerWindows, key, value); err != nil {
		return fmt.Errorf("create QER meter for SEID %d, QER %d: %w", seid, id, err)
	}

	return nil
}

func (bpfObjects *BpfObjects) DeleteQerMeter(seid uint64, id uint32) error {
	key := N3N6EntrypointQerKey{Seid: seid, QerId: id}
	if err := bpfObjects.deleteTracked(bpfObjects.QerWindows, MapQerWindows, key); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
		return fmt.Errorf("delete QER meter for SEID %d, QER %d: %w", seid, id, err)
	}

	return nil
}
