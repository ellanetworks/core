// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

type TrafficClass struct {
	SST    uint8
	SD     string
	DNN    string
	FiveQI uint8
}

type TrafficClasses struct {
	mu        sync.RWMutex
	ids       map[TrafficClass]uint16
	classes   []TrafficClass
	exhausted bool
}

func NewTrafficClasses() *TrafficClasses {
	return &TrafficClasses{
		ids:     make(map[TrafficClass]uint16),
		classes: []TrafficClass{{}},
	}
}

func (t *TrafficClasses) ID(c TrafficClass) uint16 {
	t.mu.RLock()
	id, ok := t.ids[c]
	t.mu.RUnlock()

	if ok {
		return id
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if id, ok := t.ids[c]; ok {
		return id
	}

	if len(t.classes) >= ebpf.MaxTrafficClasses {
		if !t.exhausted {
			t.exhausted = true

			logger.UpfLog.Warn("traffic class table is full; new classes are not counted per class",
				zap.Int("capacity", ebpf.MaxTrafficClasses-1),
				zap.Uint8("sst", c.SST), zap.String("sd", c.SD), zap.String("dnn", c.DNN), zap.Uint8("5qi", c.FiveQI))
		}

		return 0
	}

	id = uint16(len(t.classes))
	t.classes = append(t.classes, c)
	t.ids[c] = id

	return id
}

func (t *TrafficClasses) All() map[uint16]TrafficClass {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make(map[uint16]TrafficClass, len(t.classes)-1)

	for id, c := range t.classes[1:] {
		out[uint16(id+1)] = c
	}

	return out
}
