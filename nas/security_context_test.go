// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"testing"
)

var testKey = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// TestNewSecurityContextRejects checks that a context which could not protect
// anything is refused at construction rather than at first use.
func TestNewSecurityContextRejects(t *testing.T) {
	tests := []struct {
		name string
		opts SecurityContextOptions
		want error
	}{
		{"unimplemented integrity", SecurityContextOptions{
			Integrity: IntegrityZUC, Ciphering: CipheringAES, IntegrityKey: testKey, CipherKey: testKey,
		}, ErrUnsupportedAlgorithm},
		{"unimplemented ciphering", SecurityContextOptions{
			Integrity: IntegrityAES, Ciphering: CipheringZUC, IntegrityKey: testKey, CipherKey: testKey,
		}, ErrUnsupportedAlgorithm},
		{"unassigned identifier", SecurityContextOptions{
			Integrity: IntegrityAlgorithm(7), Ciphering: CipheringAES, IntegrityKey: testKey, CipherKey: testKey,
		}, ErrUnsupportedAlgorithm},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSecurityContext(tc.opts); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	// Null integrity leaves every message forgeable, so it takes an explicit say-so.
	null := SecurityContextOptions{Integrity: IntegrityNull, Ciphering: CipheringNull, IntegrityKey: testKey}
	if _, err := NewSecurityContext(null); err == nil {
		t.Error("null integrity was accepted without AllowNullIntegrity")
	}

	null.AllowNullIntegrity = true
	if _, err := NewSecurityContext(null); err != nil {
		t.Errorf("null integrity with AllowNullIntegrity: %v", err)
	}

	// An all-zero key is what an uninitialised context looks like.
	if _, err := NewSecurityContext(SecurityContextOptions{
		Integrity: IntegrityAES, Ciphering: CipheringAES, CipherKey: testKey,
	}); err == nil {
		t.Error("an all-zero K_NASint was accepted")
	}

	if _, err := NewSecurityContext(SecurityContextOptions{
		Integrity: IntegrityAES, Ciphering: CipheringAES, IntegrityKey: testKey,
	}); err == nil {
		t.Error("an all-zero K_NASenc was accepted under a ciphering algorithm")
	}

	// Under the null cipher there is no K_NASenc to demand.
	if _, err := NewSecurityContext(SecurityContextOptions{
		Integrity: IntegrityAES, Ciphering: CipheringNull, IntegrityKey: testKey,
	}); err != nil {
		t.Errorf("null ciphering with no K_NASenc: %v", err)
	}
}

// TestUnsupportedAlgorithmFailsClosed walks the whole 3-bit identifier space of
// both algorithm types (TS 24.501 §9.11.3.34, TS 24.301 §9.9.3.23). Every
// identifier this library does not implement — 128-NIA3/128-NEA3 and the values
// 3GPP has not assigned — must report ErrUnsupportedAlgorithm and hand back an
// implementation that errors on use, so a caller that ignored the error still
// cannot send or accept an unprotected message. The null algorithms stay
// selectable, since an operator may deliberately choose NIA0/NEA0.
func TestUnsupportedAlgorithmFailsClosed(t *testing.T) {
	implementedIntegrity := map[IntegrityAlgorithm]bool{IntegrityNull: true, IntegritySNOW3G: true, IntegrityAES: true}
	implementedCiphering := map[CipheringAlgorithm]bool{CipheringNull: true, CipheringSNOW3G: true, CipheringAES: true}

	for id := range 8 {
		alg := IntegrityAlgorithm(id)

		integ, err := IntegrityFor(alg)
		if implementedIntegrity[alg] {
			if err != nil {
				t.Errorf("IntegrityFor(%s) = %v, want an implementation", alg, err)
			}

			continue
		}

		if !errors.Is(err, ErrUnsupportedAlgorithm) {
			t.Errorf("IntegrityFor(%s) err = %v, want ErrUnsupportedAlgorithm", alg, err)
		}

		if _, err := integ.MAC(testKey, 0, Bearer3GPP, DirectionUplink, nil); !errors.Is(err, ErrUnsupportedAlgorithm) {
			t.Errorf("MAC under %s = %v, want an error", alg, err)
		}
	}

	for id := range 8 {
		alg := CipheringAlgorithm(id)

		ciph, err := CipherFor(alg)
		if implementedCiphering[alg] {
			if err != nil {
				t.Errorf("CipherFor(%s) = %v, want an implementation", alg, err)
			}

			continue
		}

		if !errors.Is(err, ErrUnsupportedAlgorithm) {
			t.Errorf("CipherFor(%s) err = %v, want ErrUnsupportedAlgorithm", alg, err)
		}

		if _, err := ciph.Apply(testKey, 0, Bearer3GPP, DirectionUplink, []byte{1}); !errors.Is(err, ErrUnsupportedAlgorithm) {
			t.Errorf("Apply under %s = %v, want an error", alg, err)
		}
	}
}

// TestSecurityContextNilIsUnusable checks that the zero value protects nothing.
func TestSecurityContextNilIsUnusable(t *testing.T) {
	var sc *SecurityContext

	if _, err := sc.MAC(nil, 0, Bearer3GPP, DirectionUplink); !errors.Is(err, ErrNoSecurityContext) {
		t.Errorf("MAC = %v, want ErrNoSecurityContext", err)
	}

	if _, err := sc.Cipher(nil, 0, Bearer3GPP, DirectionUplink); !errors.Is(err, ErrNoSecurityContext) {
		t.Errorf("Cipher = %v, want ErrNoSecurityContext", err)
	}

	if err := sc.VerifyMAC(nil, [4]byte{}, 0, Bearer3GPP, DirectionUplink); !errors.Is(err, ErrNoSecurityContext) {
		t.Errorf("VerifyMAC = %v, want ErrNoSecurityContext", err)
	}

	if sc.Ciphers() {
		t.Error("a nil context reports that it ciphers")
	}
}

// TestDownlinkCounterRefusesToWrap checks that the sender stops rather than
// reusing a NAS COUNT under the same key (TS 33.501 §6.4.3.1, TS 33.401 §6.5).
func TestDownlinkCounterRefusesToWrap(t *testing.T) {
	d := NewDownlinkCounter(MakeCount(0xFFFF, 0xFE))

	first, err := d.Use()
	if err != nil || first != MakeCount(0xFFFF, 0xFE) {
		t.Fatalf("Use() = %#06x, %v", uint32(first), err)
	}

	last, err := d.Use()
	if err != nil || last != MakeCount(0xFFFF, 0xFF) {
		t.Fatalf("Use() = %#06x, %v", uint32(last), err)
	}

	if !d.Exhausted() {
		t.Error("Exhausted() is false after the last count was used")
	}

	if _, err := d.Use(); !errors.Is(err, ErrCountExhausted) {
		t.Errorf("Use() after exhaustion = %v, want ErrCountExhausted", err)
	}

	d.Reset()

	if d.Exhausted() {
		t.Error("Exhausted() is true after Reset")
	}

	if got, err := d.Use(); err != nil || got != 0 {
		t.Errorf("Use() after Reset = %#06x, %v, want the context to restart at zero", uint32(got), err)
	}
}

// TestDownlinkCounterAdvances checks the ordinary sequence.
func TestDownlinkCounterAdvances(t *testing.T) {
	var d DownlinkCounter

	for want := range 3 {
		if d.Next() != Count(want) {
			t.Fatalf("Next() = %d, want %d", d.Next(), want)
		}

		got, err := d.Use()
		if err != nil || got != Count(want) {
			t.Fatalf("Use() = %d, %v, want %d", got, err, want)
		}
	}
}
