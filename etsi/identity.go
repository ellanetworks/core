package etsi

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"sync"
)

var maxMsbValue = big.NewInt(0x003FFFFF)

// TMSI represents a 5G Temporary Mobile Subscriber's Identity,
// as define by 3GPP.
type TMSI struct {
	tmsi uint32
}

var InvalidTMSI TMSI = TMSI{math.MaxUint32}

// NewTMSI creates a TMSI instance from a uint32 value.
// It returns an error if the resulting TMSI is invalid.
func NewTMSI(v uint32) (TMSI, error) {
	if v == math.MaxUint32 {
		return TMSI{v}, fmt.Errorf("invalid TMSI")
	}

	return TMSI{v}, nil
}

// String returns the TMSI has an hexadecimal string
func (t *TMSI) String() string {
	return fmt.Sprintf("%08x", t.tmsi)
}

// TmsiAllocator allocates and frees TMSI. It keeps
// track internally of allocated TMSI. Generated TMSIs
// are round-robined over the 10 least significant bits to spread
// paging load, as define in TS 23.501. The TMSIs are
// otherwise allocated randomly to ensure privacy.
type TmsiAllocator struct {
	allocated map[TMSI]bool
	nextLsb   uint32

	sync.Mutex
}

// NewTMSIAllocator returns a new allocator, that will allocate unique TMSI
// on demand.
func NewTMSIAllocator() *TmsiAllocator {
	ta := TmsiAllocator{
		allocated: make(map[TMSI]bool),
		nextLsb:   0,
	}

	return &ta
}

// Allocate returns a valid TMSI, or an error if the provided context
// expires.
func (ta *TmsiAllocator) Allocate(ctx context.Context) (TMSI, error) {
	for {
		select {
		case <-ctx.Done():
			return InvalidTMSI, ctx.Err()
		default:
		}

		msb, err := rand.Int(rand.Reader, maxMsbValue)
		if err != nil {
			continue
		}

		ta.Lock()

		lsb := ta.nextLsb

		t, err := NewTMSI(uint32(msb.Int64()<<10) + lsb)
		if err != nil {
			ta.Unlock()
			continue
		}

		if !ta.tryAllocate(t) {
			ta.Unlock()
			continue
		}

		ta.Unlock()

		return t, nil
	}
}

// Free returns the TMSI to the pool, allowing it to be reallocated.
func (ta *TmsiAllocator) Free(t TMSI) {
	ta.Lock()
	defer ta.Unlock()

	delete(ta.allocated, t)
}

func (ta *TmsiAllocator) tryAllocate(t TMSI) bool {
	if ta.allocated[t] {
		return false
	}

	ta.allocated[t] = true
	ta.nextLsb++

	return true
}
