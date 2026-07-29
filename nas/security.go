// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"crypto/subtle"
	"errors"
	"fmt"
)

// Direction is the direction of a NAS message, the DIRECTION input to the
// integrity and ciphering algorithms (TS 33.501 §D.3.1, TS 33.401 §B.2.3).
type Direction uint8

// NAS message directions (TS 33.501 §D.3.1, TS 33.401 §B.2.3).
const (
	DirectionUplink   Direction = 0
	DirectionDownlink Direction = 1
)

func (d Direction) String() string {
	switch d {
	case DirectionUplink:
		return "uplink"
	case DirectionDownlink:
		return "downlink"
	default:
		return fmt.Sprintf("unknown direction (%d)", uint8(d))
	}
}

// Bearer is the BEARER input to the NAS integrity and ciphering algorithms. The
// two generations give it different values, so each generation package applies
// its own.
type Bearer uint8

// NAS bearers.
const (
	// BearerEPS is the EPS value: NAS has no radio bearer, so the input is the
	// constant 0 (TS 33.401 §8.1.1).
	BearerEPS Bearer = 0x00
	// Bearer3GPP is the 5GS value for 3GPP access: the NAS connection identifier
	// (TS 33.501 §6.4.3.1, §6.9.2.1).
	Bearer3GPP Bearer = 0x01
)

// IntegrityKey is K_NASint, the 128-bit NAS integrity key (TS 33.501 §A.8,
// TS 33.401 §A.7).
type IntegrityKey [16]byte

// CipherKey is K_NASenc, the 128-bit NAS ciphering key (TS 33.501 §A.8,
// TS 33.401 §A.7).
type CipherKey [16]byte

// IntegrityAlgorithm identifies a NAS integrity algorithm. 5GS names them
// 128-NIAn (TS 33.501 §9.4.2.3) and EPS 128-EIAn (TS 33.401 §5.1.4.2); the
// identifiers and the algorithms behind them are the same.
type IntegrityAlgorithm uint8

// NAS integrity algorithms. ZUC is defined by both specs but not implemented
// here; selecting it fails closed.
const (
	IntegrityNull   IntegrityAlgorithm = 0x00 // NIA0 / EIA0
	IntegritySNOW3G IntegrityAlgorithm = 0x01 // 128-NIA1 / 128-EIA1
	IntegrityAES    IntegrityAlgorithm = 0x02 // 128-NIA2 / 128-EIA2
	IntegrityZUC    IntegrityAlgorithm = 0x03 // 128-NIA3 / 128-EIA3
)

func (a IntegrityAlgorithm) String() string {
	switch a {
	case IntegrityNull:
		return "NIA0/EIA0 (null)"
	case IntegritySNOW3G:
		return "128-NIA1/128-EIA1 (SNOW 3G)"
	case IntegrityAES:
		return "128-NIA2/128-EIA2 (AES)"
	case IntegrityZUC:
		return "128-NIA3/128-EIA3 (ZUC)"
	default:
		return fmt.Sprintf("unknown integrity algorithm (%#x)", uint8(a))
	}
}

// CipheringAlgorithm identifies a NAS ciphering algorithm. 5GS names them
// 128-NEAn (TS 33.501 §9.4.2.3) and EPS 128-EEAn (TS 33.401 §5.1.3.2); the
// identifiers and the algorithms behind them are the same.
type CipheringAlgorithm uint8

// NAS ciphering algorithms. ZUC is defined by both specs but not implemented
// here; selecting it fails closed.
const (
	CipheringNull   CipheringAlgorithm = 0x00 // NEA0 / EEA0
	CipheringSNOW3G CipheringAlgorithm = 0x01 // 128-NEA1 / 128-EEA1
	CipheringAES    CipheringAlgorithm = 0x02 // 128-NEA2 / 128-EEA2
	CipheringZUC    CipheringAlgorithm = 0x03 // 128-NEA3 / 128-EEA3
)

func (a CipheringAlgorithm) String() string {
	switch a {
	case CipheringNull:
		return "NEA0/EEA0 (null)"
	case CipheringSNOW3G:
		return "128-NEA1/128-EEA1 (SNOW 3G)"
	case CipheringAES:
		return "128-NEA2/128-EEA2 (AES)"
	case CipheringZUC:
		return "128-NEA3/128-EEA3 (ZUC)"
	default:
		return fmt.Sprintf("unknown ciphering algorithm (%#x)", uint8(a))
	}
}

// ErrUnsupportedAlgorithm reports an algorithm identifier this library does not
// implement. The implementation returned alongside it fails every operation, so
// a caller that ignores the error still cannot send or accept unprotected NAS.
var ErrUnsupportedAlgorithm = errors.New("unsupported NAS security algorithm")

// ErrMACMismatch reports a NAS-MAC that does not verify.
var ErrMACMismatch = errors.New("NAS-MAC verification failed")

// ErrNoSecurityContext reports a nil or uninitialised security context where one
// was required.
var ErrNoSecurityContext = errors.New("no NAS security context")

// Integrity computes the 4-octet NAS-MAC over a NAS message (TS 33.501 §D.3.1,
// TS 33.401 §B.2.3). Only this package implements it: an algorithm reaches a
// message through [SecurityContext], which is the only way to obtain one.
type Integrity interface {
	MAC(key IntegrityKey, count uint32, bearer Bearer, direction Direction, msg []byte) ([4]byte, error)

	isIntegrity()
}

// Cipher enciphers or deciphers a NAS payload — a keystream XOR, so the same
// operation runs in both directions (TS 33.501 §D.3.1, TS 33.401 §B.2.3). Only
// this package implements it.
type Cipher interface {
	Apply(key CipherKey, count uint32, bearer Bearer, direction Direction, data []byte) ([]byte, error)

	isCipher()
}

// IntegrityFor returns the implementation of a NAS integrity algorithm.
//
// An algorithm this library does not implement — ZUC, or an identifier 3GPP has
// not assigned — yields [ErrUnsupportedAlgorithm] and an implementation that
// fails every operation, so NAS integrity cannot silently degrade to none
// (TS 33.501 §5.5.2, TS 33.401 §5.1.4).
func IntegrityFor(alg IntegrityAlgorithm) (Integrity, error) {
	switch alg {
	case IntegrityNull:
		return nullIntegrity{}, nil
	case IntegritySNOW3G:
		return snow3gIntegrity{}, nil
	case IntegrityAES:
		return aesCMACIntegrity{}, nil
	case IntegrityZUC:
		fallthrough
	default:
		return unsupportedIntegrity{alg: alg}, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

// CipherFor returns the implementation of a NAS ciphering algorithm.
//
// An algorithm this library does not implement — ZUC, or an identifier 3GPP has
// not assigned — yields [ErrUnsupportedAlgorithm] and an implementation that
// fails every operation, rather than passing data through in the clear
// (TS 33.501 §5.5.1, TS 33.401 §5.1.3).
func CipherFor(alg CipheringAlgorithm) (Cipher, error) {
	switch alg {
	case CipheringNull:
		return nullCipher{}, nil
	case CipheringSNOW3G:
		return snow3gCipher{}, nil
	case CipheringAES:
		return aesCTRCipher{}, nil
	case CipheringZUC:
		fallthrough
	default:
		return unsupportedCipher{alg: alg}, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

// SecurityContextOptions describes the NAS security context to build. Every
// field is what the security mode procedure agreed: the two algorithms and the
// two keys derived from K_AMF or K_ASME (TS 33.501 §6.7.2, TS 33.401 §7.2.4.4).
type SecurityContextOptions struct {
	Integrity IntegrityAlgorithm
	Ciphering CipheringAlgorithm

	// IntegrityKey is K_NASint. It may not be all zero: an all-zero key is what
	// an uninitialised context looks like, and MACing under it would produce a
	// value a peer could reproduce.
	IntegrityKey IntegrityKey
	// CipherKey is K_NASenc. It may not be all zero under a ciphering algorithm
	// other than the null one, and is ignored under the null one.
	CipherKey CipherKey

	// AllowNullIntegrity permits NIA0/EIA0, which leaves every message forgeable.
	// Both specs allow it only for an unauthenticated emergency session
	// (TS 33.501 §5.5.2, TS 33.401 §5.1.4.2), so a context that uses it has to
	// say so here.
	AllowNullIntegrity bool
}

// SecurityContext is a NAS security context: the algorithms the security mode
// procedure agreed and the keys derived for them. It is the only way to obtain
// an [Integrity] or a [Cipher], so every protected message goes through a
// context a caller had to build deliberately.
//
// A context holds no counters. The NAS COUNTs belong to the connection, not to
// the keys: see [DownlinkCounter] and [UplinkCounter].
//
// The zero value is unusable; build one with [NewSecurityContext]. A context is
// immutable once built, so concurrent use is safe.
type SecurityContext struct {
	integrityAlg IntegrityAlgorithm
	cipheringAlg CipheringAlgorithm
	integ        Integrity
	ciph         Cipher
	kNASint      IntegrityKey
	kNASenc      CipherKey
}

// NewSecurityContext builds a NAS security context, rejecting one that could not
// protect anything: an algorithm this library does not implement, a key left all
// zero, or null integrity that the caller did not ask for.
func NewSecurityContext(opts SecurityContextOptions) (*SecurityContext, error) {
	integ, err := IntegrityFor(opts.Integrity)
	if err != nil {
		return nil, err
	}

	ciph, err := CipherFor(opts.Ciphering)
	if err != nil {
		return nil, err
	}

	if opts.Integrity == IntegrityNull && !opts.AllowNullIntegrity {
		return nil, fmt.Errorf(
			"nas: %s leaves every message forgeable; set AllowNullIntegrity to use it", opts.Integrity)
	}

	if opts.IntegrityKey == (IntegrityKey{}) {
		return nil, errors.New("nas: K_NASint is all zero")
	}

	if opts.Ciphering != CipheringNull && opts.CipherKey == (CipherKey{}) {
		return nil, fmt.Errorf("nas: K_NASenc is all zero under %s", opts.Ciphering)
	}

	return &SecurityContext{
		integrityAlg: opts.Integrity,
		cipheringAlg: opts.Ciphering,
		integ:        integ,
		ciph:         ciph,
		kNASint:      opts.IntegrityKey,
		kNASenc:      opts.CipherKey,
	}, nil
}

// IntegrityAlgorithm returns the agreed NAS integrity algorithm.
func (sc *SecurityContext) IntegrityAlgorithm() IntegrityAlgorithm { return sc.integrityAlg }

// CipheringAlgorithm returns the agreed NAS ciphering algorithm.
func (sc *SecurityContext) CipheringAlgorithm() CipheringAlgorithm { return sc.cipheringAlg }

// Ciphers reports whether the context enciphers at all: it does not under
// NEA0/EEA0, whatever the security header type asks for.
func (sc *SecurityContext) Ciphers() bool { return sc != nil && sc.cipheringAlg != CipheringNull }

func (sc *SecurityContext) String() string {
	if sc == nil {
		return "no NAS security context"
	}

	return fmt.Sprintf("%s / %s", sc.integrityAlg, sc.cipheringAlg)
}

// MAC computes the NAS-MAC over msg (TS 33.501 §D.3.1, TS 33.401 §B.2.3).
//
// It is the mechanism the generation packages' Protect uses; call that instead
// unless the message carries no security header of its own — the NAS message
// container of a SECURITY MODE COMPLETE, or the short MAC of an EPS SERVICE
// REQUEST.
func (sc *SecurityContext) MAC(msg []byte, count Count, bearer Bearer, dir Direction) ([4]byte, error) {
	if sc == nil {
		return [4]byte{}, ErrNoSecurityContext
	}

	return sc.integ.MAC(sc.kNASint, count.Value(), bearer, dir, msg)
}

// VerifyMAC checks a received NAS-MAC against the one msg produces, in constant
// time. It returns [ErrMACMismatch] when they differ.
func (sc *SecurityContext) VerifyMAC(msg []byte, got [4]byte, count Count, bearer Bearer, dir Direction) error {
	want, err := sc.MAC(msg, count, bearer, dir)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		return ErrMACMismatch
	}

	return nil
}

// Cipher enciphers or deciphers data — the transform is its own inverse
// (TS 33.501 §D.3.1, TS 33.401 §B.2.3).
//
// It is the mechanism the generation packages' Protect and Unprotect use; call
// that instead unless the data carries no security header of its own, such as a
// NAS message container ciphered on its own count.
func (sc *SecurityContext) Cipher(data []byte, count Count, bearer Bearer, dir Direction) ([]byte, error) {
	if sc == nil {
		return nil, ErrNoSecurityContext
	}

	return sc.ciph.Apply(sc.kNASenc, count.Value(), bearer, dir, data)
}

// nullIntegrity is NIA0/EIA0: no integrity, an all-zero MAC. Both specs permit
// it only for an unauthenticated emergency session (TS 33.501 §5.5.2,
// TS 33.401 §5.1.4.2).
type nullIntegrity struct{}

func (nullIntegrity) MAC(_ IntegrityKey, _ uint32, _ Bearer, _ Direction, _ []byte) ([4]byte, error) {
	return [4]byte{}, nil
}

func (nullIntegrity) isIntegrity() {}

// nullCipher is NEA0/EEA0: no ciphering, data passes through unchanged.
type nullCipher struct{}

func (nullCipher) Apply(_ CipherKey, _ uint32, _ Bearer, _ Direction, data []byte) ([]byte, error) {
	out := make([]byte, len(data))
	copy(out, data)

	return out, nil
}

func (nullCipher) isCipher() {}

// unsupportedIntegrity is the fail-closed integrity for an algorithm this
// library does not implement: every MAC errors, so the protection path aborts
// rather than producing one a peer would reject or an all-zero one it would not.
type unsupportedIntegrity struct{ alg IntegrityAlgorithm }

func (u unsupportedIntegrity) MAC(_ IntegrityKey, _ uint32, _ Bearer, _ Direction, _ []byte) ([4]byte, error) {
	return [4]byte{}, fmt.Errorf("nas: %w: %s", ErrUnsupportedAlgorithm, u.alg)
}

func (unsupportedIntegrity) isIntegrity() {}

// unsupportedCipher is the fail-closed cipher for an algorithm this library does
// not implement: every operation errors rather than passing data through in the
// clear.
type unsupportedCipher struct{ alg CipheringAlgorithm }

func (u unsupportedCipher) Apply(_ CipherKey, _ uint32, _ Bearer, _ Direction, _ []byte) ([]byte, error) {
	return nil, fmt.Errorf("nas: %w: %s", ErrUnsupportedAlgorithm, u.alg)
}

func (unsupportedCipher) isCipher() {}
