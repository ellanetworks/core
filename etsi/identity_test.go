package etsi_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func TestNewTMSI_Invalid(t *testing.T) {
	expected := "invalid TMSI"

	_, err := etsi.NewTMSI(0xFFFFFFFF)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected invalid TMSI error, got: %v", err)
	}
}

func TestNewTMSI_Valid(t *testing.T) {
	testcases := []uint32{0, 1, 2, 42, 0xFFFFFFFE}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("TMSI_%8x", tc), func(t *testing.T) {
			_, err := etsi.NewTMSI(tc)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func TestTMSIAllocator_AllocateAndClose(t *testing.T) {
	ta := etsi.NewTMSIAllocator(t.Context(), 1)

	tmsi, err := ta.Allocate(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if tmsi == etsi.InvalidTMSI {
		t.Fatalf("expected valid TMSI, got %v", tmsi)
	}

	err = ta.Close()
	if err != nil {
		t.Fatalf("expected no error closing allocator, got %v", err)
	}
}

func TestTMSIAllocator_AllocateBalanceLSB(t *testing.T) {
	ta := etsi.NewTMSIAllocator(t.Context(), 1)

	defer func() { _ = ta.Close() }()

	dist := make(map[string]int)

	for range 2048 {
		tmsi, err := ta.Allocate(t.Context())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		ntmsi, err := strconv.ParseUint(tmsi.String(), 16, 32)
		if err != nil {
			t.Fatalf("could not parse TMSI: %v", err)
		}

		lsb := strconv.FormatUint(ntmsi, 2)

		dist[lsb[len(lsb)-10:]] += 1
	}

	for k, v := range dist {
		if v != 2 {
			t.Fatalf("count for lsb %v was not balanced: %d", k, v)
		}
	}
}

func BenchmarkTMSIAllocation(b *testing.B) {
	counts := []int{100, 1000, 10000, 100000, 1000000}

	for _, count := range counts {
		b.Run(fmt.Sprintf("TMSI-Count-%d", count), func(b *testing.B) {
			ta := etsi.NewTMSIAllocator(b.Context(), 20)

			defer func() { _ = ta.Close() }()

			for b.Loop() {
				for range count {
					_, err := ta.Allocate(b.Context())
					if err != nil {
						b.Fatalf("expected no error, got %v", err)
					}
				}
			}
		})
	}
}

func BenchmarkTMSIAllocationAndFree(b *testing.B) {
	counts := []int{100, 1000, 10000, 100000, 1000000}

	for _, count := range counts {
		b.Run(fmt.Sprintf("TMSI-Count-%d", count), func(b *testing.B) {
			ta := etsi.NewTMSIAllocator(b.Context(), 20)

			defer func() { _ = ta.Close() }()

			for b.Loop() {
				for range count {
					tmsi, err := ta.Allocate(b.Context())
					if err != nil {
						b.Fatalf("expected no error, got %v", err)
					}

					ta.Free(tmsi)
				}
			}
		})
	}
}

func BenchmarkTMSIConcurrentAllocationAndFree(b *testing.B) {
	preallocate := []uint{1, 10, 20, 50, 100, 1000}

	for _, p := range preallocate {
		b.Run(fmt.Sprintf("Preallocate-%d", p), func(b *testing.B) {
			ta := etsi.NewTMSIAllocator(b.Context(), p)

			defer func() { _ = ta.Close() }()

			// Simulate 1000 subscribers
			b.SetParallelism(1000)

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					tmsi, err := ta.Allocate(context.TODO())
					if err != nil {
						b.Fatalf("expected no error, got %v", err)
					}

					ta.Free(tmsi)
				}
			})
		})
	}
}
