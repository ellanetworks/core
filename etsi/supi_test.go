package etsi_test

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func TestNewSUPIFromIMSI(t *testing.T) {
	type Testcase struct {
		name     string
		imsi     string
		expected string
	}

	testcases := []Testcase{
		{"FiveDigits", "00101", "imsi-00101"},
		{"TenDigits", "0010197561", "imsi-0010197561"},
		{"FifteenDigits", "001019756139935", "imsi-001019756139935"},
		{"AllZeros", "00000", "imsi-00000"},
		{"AllNines", "999999999999999", "imsi-999999999999999"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			supi, err := etsi.NewSUPIFromIMSI(tc.imsi)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if supi.String() != tc.expected {
				t.Fatalf("expected String(): %s, got: %s", tc.expected, supi.String())
			}

			if supi.IMSI() != tc.imsi {
				t.Fatalf("expected IMSI(): %s, got: %s", tc.imsi, supi.IMSI())
			}
		})
	}
}

func TestNewSUPIFromIMSIInvalid(t *testing.T) {
	type Testcase struct {
		name     string
		imsi     string
		expected string
	}

	testcases := []Testcase{
		{"Empty", "", "invalid IMSI length: 0 (must be 5-15 digits)"},
		{"TooShort", "0010", "invalid IMSI length: 4 (must be 5-15 digits)"},
		{"TooLong", "0010197561399350", "invalid IMSI length: 16 (must be 5-15 digits)"},
		{"ContainsLetters", "0010a975613", "invalid IMSI: contains non-digit characters: 0010a975613"},
		{"ContainsHyphen", "001-01-9756", "invalid IMSI: contains non-digit characters: 001-01-9756"},
		{"ContainsSpace", "00101 97561", "invalid IMSI: contains non-digit characters: 00101 97561"},
		{"ContainsPlus", "+0010197561", "invalid IMSI: contains non-digit characters: +0010197561"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			supi, err := etsi.NewSUPIFromIMSI(tc.imsi)
			if err == nil || err.Error() != tc.expected {
				t.Fatalf("expected error: %s, got: %v", tc.expected, err)
			}

			if supi != etsi.InvalidSUPI {
				t.Fatalf("expected InvalidSUPI, got: %s", supi.String())
			}
		})
	}
}

func TestNewSUPIFromPrefixed(t *testing.T) {
	type Testcase struct {
		name         string
		input        string
		expectedIMSI string
	}

	testcases := []Testcase{
		{"WithPrefix", "imsi-001019756139935", "001019756139935"},
		{"WithoutPrefix", "001019756139935", "001019756139935"},
		{"ShortWithPrefix", "imsi-00101", "00101"},
		{"ShortWithoutPrefix", "00101", "00101"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			supi, err := etsi.NewSUPIFromPrefixed(tc.input)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if supi.IMSI() != tc.expectedIMSI {
				t.Fatalf("expected IMSI(): %s, got: %s", tc.expectedIMSI, supi.IMSI())
			}
		})
	}
}

func TestNewSUPIFromPrefixedInvalid(t *testing.T) {
	type Testcase struct {
		name     string
		input    string
		expected string
	}

	testcases := []Testcase{
		{"NaiPrefix", "nai-user@realm", "invalid IMSI: contains non-digit characters: nai-user@realm"},
		{"SuciPrefix", "suci-0-001-01-0000-0-0-001019756139935", "invalid IMSI length: 38 (must be 5-15 digits)"},
		{"Garbage", "not-a-supi", "invalid IMSI: contains non-digit characters: not-a-supi"},
		{"ImsiPrefixNoDigits", "imsi-", "invalid IMSI length: 0 (must be 5-15 digits)"},
		{"ImsiPrefixTooShort", "imsi-001", "invalid IMSI length: 3 (must be 5-15 digits)"},
		{"EmptyString", "", "invalid IMSI length: 0 (must be 5-15 digits)"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			supi, err := etsi.NewSUPIFromPrefixed(tc.input)
			if err == nil || err.Error() != tc.expected {
				t.Fatalf("expected error: %s, got: %v", tc.expected, err)
			}

			if supi != etsi.InvalidSUPI {
				t.Fatalf("expected InvalidSUPI, got: %s", supi.String())
			}
		})
	}
}

func TestNewSUPIFromNAI(t *testing.T) {
	expected := "NAI SUPI not yet supported"

	supi, err := etsi.NewSUPIFromNAI("user@realm")
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}

	if supi != etsi.InvalidSUPI {
		t.Fatalf("expected InvalidSUPI, got: %s", supi.String())
	}
}

func TestSUPIString(t *testing.T) {
	type Testcase struct {
		name     string
		supi     etsi.SUPI
		expected string
	}

	validSUPI, _ := etsi.NewSUPIFromIMSI("001019756139935")

	testcases := []Testcase{
		{"ValidIMSI", validSUPI, "imsi-001019756139935"},
		{"ZeroValue", etsi.SUPI{}, ""},
		{"InvalidSUPI", etsi.InvalidSUPI, ""},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.supi.String() != tc.expected {
				t.Fatalf("expected String(): %q, got: %q", tc.expected, tc.supi.String())
			}
		})
	}
}

func TestSUPIIMSI(t *testing.T) {
	type Testcase struct {
		name     string
		supi     etsi.SUPI
		expected string
	}

	validSUPI, _ := etsi.NewSUPIFromIMSI("001019756139935")
	shortSUPI, _ := etsi.NewSUPIFromIMSI("00101")

	testcases := []Testcase{
		{"FullIMSI", validSUPI, "001019756139935"},
		{"ShortIMSI", shortSUPI, "00101"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.supi.IMSI() != tc.expected {
				t.Fatalf("expected IMSI(): %q, got: %q", tc.expected, tc.supi.IMSI())
			}
		})
	}
}

func TestSUPIIMSIPanicOnZero(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic, got none")
		}

		msg, ok := r.(string)
		if !ok || msg != "IMSI() called on non-IMSI SUPI" {
			t.Fatalf("expected panic message: IMSI() called on non-IMSI SUPI, got: %v", r)
		}
	}()

	var zero etsi.SUPI
	zero.IMSI()
}

func TestSUPIIsValid(t *testing.T) {
	type Testcase struct {
		name     string
		supi     etsi.SUPI
		expected bool
	}

	validSUPI, _ := etsi.NewSUPIFromIMSI("001019756139935")

	testcases := []Testcase{
		{"ConstructedIMSI", validSUPI, true},
		{"ZeroValue", etsi.SUPI{}, false},
		{"InvalidSUPI", etsi.InvalidSUPI, false},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.supi.IsValid() != tc.expected {
				t.Fatalf("expected IsValid(): %v, got: %v", tc.expected, tc.supi.IsValid())
			}
		})
	}
}

func TestSUPIIsIMSI(t *testing.T) {
	type Testcase struct {
		name     string
		supi     etsi.SUPI
		expected bool
	}

	validSUPI, _ := etsi.NewSUPIFromIMSI("001019756139935")

	testcases := []Testcase{
		{"ConstructedIMSI", validSUPI, true},
		{"ZeroValue", etsi.SUPI{}, false},
		{"InvalidSUPI", etsi.InvalidSUPI, false},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.supi.IsIMSI() != tc.expected {
				t.Fatalf("expected IsIMSI(): %v, got: %v", tc.expected, tc.supi.IsIMSI())
			}
		})
	}
}

func TestSUPIIsNAI(t *testing.T) {
	type Testcase struct {
		name     string
		supi     etsi.SUPI
		expected bool
	}

	validSUPI, _ := etsi.NewSUPIFromIMSI("001019756139935")

	testcases := []Testcase{
		{"ConstructedIMSI", validSUPI, false},
		{"ZeroValue", etsi.SUPI{}, false},
		{"InvalidSUPI", etsi.InvalidSUPI, false},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.supi.IsNAI() != tc.expected {
				t.Fatalf("expected IsNAI(): %v, got: %v", tc.expected, tc.supi.IsNAI())
			}
		})
	}
}

func TestSUPIMapKey(t *testing.T) {
	fromBare, err := etsi.NewSUPIFromIMSI("001019756139935")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	fromPrefixed, err := etsi.NewSUPIFromPrefixed("imsi-001019756139935")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fromBare != fromPrefixed {
		t.Fatalf("expected equal SUPIs, got: %s != %s", fromBare.String(), fromPrefixed.String())
	}

	// Verify they work as map keys and resolve to the same entry
	m := make(map[etsi.SUPI]string)
	m[fromBare] = "subscriber1"

	val, ok := m[fromPrefixed]
	if !ok {
		t.Fatalf("expected to find entry using prefixed SUPI as key")
	}

	if val != "subscriber1" {
		t.Fatalf("expected value: subscriber1, got: %s", val)
	}
}

func TestSUPIEquality(t *testing.T) {
	type Testcase struct {
		name     string
		a        etsi.SUPI
		b        etsi.SUPI
		expected bool
	}

	supi1, _ := etsi.NewSUPIFromIMSI("001019756139935")
	supi2, _ := etsi.NewSUPIFromIMSI("001019756139935")
	supi3, _ := etsi.NewSUPIFromIMSI("001019756139936")

	testcases := []Testcase{
		{"SameValue", supi1, supi2, true},
		{"DifferentValue", supi1, supi3, false},
		{"BothZero", etsi.SUPI{}, etsi.InvalidSUPI, true},
		{"ValidVsZero", supi1, etsi.SUPI{}, false},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a == tc.b
			if result != tc.expected {
				t.Fatalf("expected %s == %s to be %v, got %v", tc.a.String(), tc.b.String(), tc.expected, result)
			}
		})
	}
}
