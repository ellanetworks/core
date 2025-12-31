// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

// idgenerator is used for generating ID from minValue to maxValue.
// It will allocate IDs in range [minValue, maxValue]
// It is thread-safe when allocating IDs
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

// Initialize an IDGenerator with minValue and maxValue.
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

// Allocate and return an id in range [minValue, maxValue]
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
