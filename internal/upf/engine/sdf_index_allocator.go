package engine

import (
	"fmt"
	"sync"
)

// SdfIndexAllocator manages allocation of slots in the sdf_filters BPF array.
// Index 0 is permanently reserved as "no filter".
type SdfIndexAllocator struct {
	mu   sync.Mutex
	free []uint32
	used map[uint32]struct{}
}

func NewSdfIndexAllocator(maxSlots uint32) *SdfIndexAllocator {
	free := make([]uint32, 0, maxSlots-1)
	for i := uint32(1); i < maxSlots; i++ {
		free = append(free, i)
	}

	return &SdfIndexAllocator{
		free: free,
		used: make(map[uint32]struct{}),
	}
}

func (a *SdfIndexAllocator) Allocate() (uint32, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.free) == 0 {
		return 0, fmt.Errorf("sdf filter index pool exhausted")
	}

	idx := a.free[len(a.free)-1]
	a.free = a.free[:len(a.free)-1]
	a.used[idx] = struct{}{}

	return idx, nil
}

func (a *SdfIndexAllocator) Release(idx uint32) {
	if idx == 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.used[idx]; ok {
		delete(a.used, idx)
		a.free = append(a.free, idx)
	}
}
