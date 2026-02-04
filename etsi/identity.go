package etsi

import (
	"container/heap"
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
// are balanced over the 10 least significant bits to spread
// paging load, as define in TS 23.501. The TMSIs are
// otherwise allocated randomly to ensure privacy.
type TmsiAllocator struct {
	allocatable  <-chan TMSI
	allocated    map[TMSI]bool
	cancel       context.CancelFunc
	lsbBuckets   [1024]*pagingBucket
	lsbPrioQueue lsbQueue

	sync.Mutex
}

// NewTMSIAllocator returns a new allocator, that will keep `preallocate` TMSIs
// ready to allocate. It can be closed by cancelling the provided context, or
// calling Close().
func NewTMSIAllocator(ctx context.Context, preallocate uint) *TmsiAllocator {
	var lsbPrioQueue lsbQueue

	c := make(chan TMSI, preallocate)
	cctx, cancel := context.WithCancel(ctx)

	lsbBuckets := [1024]*pagingBucket{}
	for i := range lsbBuckets {
		lsbBuckets[i] = &pagingBucket{lsb: uint32(i)}
		heap.Push(&lsbPrioQueue, lsbBuckets[i])
	}

	ta := TmsiAllocator{
		allocatable:  c,
		allocated:    make(map[TMSI]bool),
		cancel:       cancel,
		lsbBuckets:   lsbBuckets,
		lsbPrioQueue: lsbPrioQueue,
	}

	go ta.preallocate(cctx, c)

	return &ta
}

// Close closes the allocator
func (ta *TmsiAllocator) Close() error {
	ta.cancel()
	return nil
}

// Allocate returns a valid TMSI, or an error if the provided context
// expires.
func (ta *TmsiAllocator) Allocate(ctx context.Context) (TMSI, error) {
	select {
	case t := <-ta.allocatable:
		return t, nil
	case <-ctx.Done():
		return InvalidTMSI, ctx.Err()
	}
}

// Free returns the TMSI to the pool, allowing it to be reallocated.
func (ta *TmsiAllocator) Free(t TMSI) {
	ta.Lock()
	defer ta.Unlock()

	delete(ta.allocated, t)

	lsb := ta.lsbBuckets[t.tmsi&0x03FF]
	if lsb.count > 0 {
		lsb.count--
	}

	heap.Init(&ta.lsbPrioQueue)
}

func (ta *TmsiAllocator) preallocate(ctx context.Context, c chan<- TMSI) {
	for {
		msb, err := rand.Int(rand.Reader, maxMsbValue)
		if err != nil {
			continue
		}

		lsb := ta.nextLsb()

		t, err := NewTMSI(uint32(msb.Int64()<<10) + lsb.lsb)
		if err != nil {
			continue
		}

		if !ta.tryAllocate(t, lsb) {
			continue
		}

		select {
		case c <- t:
		case <-ctx.Done():
			close(c)
			return
		}
	}
}

func (ta *TmsiAllocator) tryAllocate(t TMSI, lsb *pagingBucket) bool {
	ta.Lock()
	defer ta.Unlock()

	if ta.allocated[t] {
		return false
	}

	ta.allocated[t] = true
	lsb.count++
	heap.Push(&ta.lsbPrioQueue, lsb)

	return true
}

func (ta *TmsiAllocator) nextLsb() *pagingBucket {
	ta.Lock()
	defer ta.Unlock()

	return heap.Pop(&ta.lsbPrioQueue).(*pagingBucket)
}

type pagingBucket struct {
	lsb   uint32
	count uint32
}

type lsbQueue []*pagingBucket

func (lq lsbQueue) Len() int {
	return len(lq)
}

func (lq lsbQueue) Less(i, j int) bool {
	return lq[i].count < lq[j].count
}

func (lq lsbQueue) Swap(i, j int) {
	lq[i], lq[j] = lq[j], lq[i]
}

func (lq *lsbQueue) Push(x any) {
	*lq = append(*lq, x.(*pagingBucket))
}

func (lq *lsbQueue) Pop() any {
	old := *lq
	n := len(old)
	x := old[n-1]
	*lq = old[0 : n-1]

	return x
}
