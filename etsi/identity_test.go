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
			tmsi, err := etsi.NewTMSI(tc)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if tmsi == etsi.InvalidTMSI {
				t.Fatalf("expected valid TMSI, got %v", tmsi)
			}
		})
	}
}

func TestTMSIAllocator_AllocateBalanceLSB(t *testing.T) {
	ta := etsi.NewTMSIAllocator()

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
			ta := etsi.NewTMSIAllocator()

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

func TestNewGUTIInvalid(t *testing.T) {
	type Testcase struct {
		name     string
		mcc      string
		mnc      string
		amfid    string
		tmsi     etsi.TMSI
		expected string
	}

	validTmsi, _ := etsi.NewTMSI(0xDEADBEEF)

	testcases := []Testcase{
		{"TooShortMCC", "01", "01", "cafe42", validTmsi, "invalid mcc: 01"},
		{"TooLongMCC", "0001", "01", "cafe42", validTmsi, "invalid mcc: 0001"},
		{"NonNumericMCC", "bee", "01", "cafe42", validTmsi, "invalid mcc: bee"},
		{"TooShortMNC", "001", "1", "cafe42", validTmsi, "invalid mnc: 1"},
		{"TooLongMNC", "001", "0001", "cafe42", validTmsi, "invalid mnc: 0001"},
		{"NonNumericMNC", "001", "bee", "cafe42", validTmsi, "invalid mnc: bee"},
		{"TooShortAMFID", "001", "01", "cafe", validTmsi, "invalid amfid: cafe"},
		{"TooLongAMFID", "001", "01", "cafe4242", validTmsi, "invalid amfid: cafe4242"},
		{"NonHexAMFID", "001", "01", "pizza1", validTmsi, "invalid amfid: pizza1"},
		{"InvalidTMSI", "001", "01", "cafe42", etsi.InvalidTMSI, "invalid tmsi: ffffffff"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			guti, err := etsi.NewGUTI(tc.mcc, tc.mnc, tc.amfid, tc.tmsi)
			if err == nil || err.Error() != tc.expected {
				t.Fatalf("expected error: %s, got: %v", tc.expected, err)
			}

			if guti != etsi.InvalidGUTI {
				t.Fatalf("expected invalid GUTI, got: %s", &guti)
			}
		})
	}
}

func TestNewGUTIValid(t *testing.T) {
	type Testcase struct {
		expected string
		mcc      string
		mnc      string
		amfid    string
		tmsi     etsi.TMSI
	}

	validTmsi, _ := etsi.NewTMSI(0xDEADBEEF)

	testcases := []Testcase{
		{"00101000001deadbeef", "001", "01", "000001", validTmsi},
		{"001001cafe42deadbeef", "001", "001", "CAFE42", validTmsi},
		{"99999cafe42deadbeef", "999", "99", "cafe42", validTmsi},
		{"30242450514deadbeef", "302", "42", "450514", validTmsi},
	}
	for _, tc := range testcases {
		t.Run(tc.expected, func(t *testing.T) {
			guti, err := etsi.NewGUTI(tc.mcc, tc.mnc, tc.amfid, tc.tmsi)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if guti.String() != tc.expected {
				t.Fatalf("expected GUTI: %s, got: %s", tc.expected, &guti)
			}
		})
	}
}

func TestNewGutiFromBytes_Invalid(t *testing.T) {
	type Testcase struct {
		name     string
		buf      []byte
		expected string
	}

	testcases := []Testcase{
		{"tooshort", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid GUTI length"},
		{"toolong", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid GUTI length"},
		{"invalidmccdigit1", []byte{0x00, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid PLMN: invalid mcc"},
		{"invalidmccdigit2", []byte{0x00, 0xa0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid PLMN: invalid mcc"},
		{"invalidmccdigit3", []byte{0x00, 0x00, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid PLMN: invalid mcc"},
		{"invalidmncdigit1", []byte{0x00, 0x00, 0xa0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid PLMN: invalid mnc"},
		{"invalidmncdigit2", []byte{0x00, 0x00, 0x00, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid PLMN: invalid mnc"},
		{"invalidmncdigit3", []byte{0x00, 0x00, 0x00, 0xa0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "invalid PLMN: invalid mnc"},
		{"invalidtmsi", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff}, "invalid TMSI"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			guti, err := etsi.NewGUTIFromBytes(tc.buf)
			if err == nil || err.Error() != tc.expected {
				t.Fatalf("expected error: %s, got: %v", tc.expected, err)
			}

			if guti != etsi.InvalidGUTI {
				t.Fatalf("expected invalid GUTI, got: %s", &guti)
			}
		})
	}
}

func TestNewGUTIFromBytes_Valid(t *testing.T) {
	type Testcase struct {
		expected string
		buf      []byte
	}

	testcases := []Testcase{
		{"00101000001deadbeef", []byte{0x00, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x01, 0xde, 0xad, 0xbe, 0xef}},
		{"001001cafe42deadbeef", []byte{0x00, 0x00, 0x11, 0x00, 0xca, 0xfe, 0x42, 0xde, 0xad, 0xbe, 0xef}},
		{"99999cafe42deadbeef", []byte{0x00, 0x99, 0xf9, 0x99, 0xca, 0xfe, 0x42, 0xde, 0xad, 0xbe, 0xef}},
		{"30242450514deadbeef", []byte{0x00, 0x03, 0xf2, 0x24, 0x45, 0x05, 0x14, 0xde, 0xad, 0xbe, 0xef}},
	}
	for _, tc := range testcases {
		t.Run(tc.expected, func(t *testing.T) {
			guti, err := etsi.NewGUTIFromBytes(tc.buf)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if guti.String() != tc.expected {
				t.Fatalf("expected GUTI: %s, got: %s", tc.expected, &guti)
			}
		})
	}
}

func BenchmarkTMSIAllocationAndFree(b *testing.B) {
	counts := []int{100, 1000, 10000, 100000, 1000000}

	for _, count := range counts {
		b.Run(fmt.Sprintf("TMSI-Count-%d", count), func(b *testing.B) {
			ta := etsi.NewTMSIAllocator()

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
	ta := etsi.NewTMSIAllocator()

	for range 1000000 {
		_, err := ta.Allocate(b.Context())
		if err != nil {
			b.Fatalf("expected no error, got %v", err)
		}
	}

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
}
