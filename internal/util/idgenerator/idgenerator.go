// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

// Package idgenerator allocates and manages IDs within a specified range.
//
// IDGenerator allocates IDs sequentially from minValue to maxValue with wrap-around
// behavior when maxValue is reached. It tracks used IDs to prevent duplicates and
// supports ID reclamation via FreeID() for true ID reuse.
//
// Thread-Safety: All operations are protected by a mutex, making IDGenerator safe
// for concurrent use.
//
// Typical Usage Example:
//
//	gen := NewGenerator(1, 65535)
//	id1, _ := gen.Allocate()  // Returns 1
//	id2, _ := gen.Allocate()  // Returns 2
//	gen.FreeID(id1)           // Marks ID 1 for reuse
//	id3, _ := gen.Allocate()  // Returns 1 (immediately reused)
//
// When resources associated with an ID are deleted, FreeID() should be called
// to mark the ID for reuse. This is critical for preventing ID exhaustion in
// long-running systems that create and destroy resources frequently.
package idgenerator

import (
	"fmt"
	"sync"
)

type IDGenerator struct {
	lock       sync.Mutex
	minValue   int64
	maxValue   int64
	valueRange int64
	offset     int64
	usedMap    map[int64]bool
}

// NewGenerator creates a new IDGenerator that allocates IDs within [minValue, maxValue].
// IDs are allocated sequentially starting from minValue, and wrap around when maxValue
// is reached. The generator is fully thread-safe.
func NewGenerator(minValue, maxValue int64) *IDGenerator {
	idGenerator := &IDGenerator{}
	idGenerator.init(minValue, maxValue)

	return idGenerator
}

func (idGenerator *IDGenerator) init(minValue, maxValue int64) {
	idGenerator.minValue = minValue
	idGenerator.maxValue = maxValue
	idGenerator.valueRange = maxValue - minValue + 1
	idGenerator.offset = 0
	idGenerator.usedMap = make(map[int64]bool)
}

// Allocate returns the next available ID in the range [minValue, maxValue].
// It skips IDs marked as used and wraps around when maxValue is exceeded.
// Returns an error if all IDs in the range are exhausted.
func (idGenerator *IDGenerator) Allocate() (int64, error) {
	idGenerator.lock.Lock()
	defer idGenerator.lock.Unlock()

	offsetBegin := idGenerator.offset
	for {
		if _, ok := idGenerator.usedMap[idGenerator.offset]; ok {
			idGenerator.updateOffset()

			if idGenerator.offset == offsetBegin {
				return 0, fmt.Errorf("no available value range to allocate id")
			}
		} else {
			break
		}
	}

	idGenerator.usedMap[idGenerator.offset] = true
	id := idGenerator.offset + idGenerator.minValue
	idGenerator.updateOffset()

	return id, nil
}

// FreeID marks an ID as available for reuse.
// Freed IDs become immediately available for the next Allocate() call.
// This is critical for preventing ID exhaustion in long-running systems.
// If the ID is outside the valid range [minValue, maxValue], it is silently ignored.
func (idGenerator *IDGenerator) FreeID(id int64) {
	if id < idGenerator.minValue || id > idGenerator.maxValue {
		return
	}

	idGenerator.lock.Lock()
	defer idGenerator.lock.Unlock()

	delete(idGenerator.usedMap, id-idGenerator.minValue)
}

func (idGenerator *IDGenerator) updateOffset() {
	idGenerator.offset++
	idGenerator.offset = idGenerator.offset % idGenerator.valueRange
}
