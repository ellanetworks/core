// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

type Radios struct {
	mu        sync.RWMutex
	ids       map[string]uint16
	radios    []string
	exhausted bool
}

func NewRadios() *Radios {
	return &Radios{
		ids:    make(map[string]uint16),
		radios: []string{""},
	}
}

func (r *Radios) ID(radio string) uint16 {
	if radio == "" {
		return 0
	}

	r.mu.RLock()
	id, ok := r.ids[radio]
	r.mu.RUnlock()

	if ok {
		return id
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.ids[radio]; ok {
		return id
	}

	if len(r.radios) >= ebpf.MaxRadios {
		if !r.exhausted {
			r.exhausted = true

			logger.UpfLog.Warn("radio table is full; new radios are not counted per radio",
				zap.Int("capacity", ebpf.MaxRadios-1), zap.String("radio", radio))
		}

		return 0
	}

	id = uint16(len(r.radios))
	r.radios = append(r.radios, radio)
	r.ids[radio] = id

	return id
}

func (r *Radios) All() map[uint16]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[uint16]string, len(r.radios)-1)

	for id, radio := range r.radios[1:] {
		out[uint16(id+1)] = radio
	}

	return out
}
