// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nas implements the primitives and information elements shared by the
// 4G and 5G NAS control planes, on which the per-generation packages
// [github.com/ellanetworks/core/nas/eps] (EPS, TS 24.301) and
// [github.com/ellanetworks/core/nas/fgs] (5GS, TS 24.501) are built.
//
// It carries the TS 24.007 message-format machinery — a bounds-checked octet
// [Reader], an appending [Writer], the length-prefixed value helpers (LV with a
// one-octet length, LV-E with a two-octet length), and [Walker] for a message's
// variable part — together with NAS security (the integrity and ciphering
// algorithms, [Count] and [UplinkCounter]), TBCD and PLMN coding, and the
// information elements 3GPP defines identically for both generations.
//
// Nothing here is specific to one generation, and this package never imports
// the generation packages.
//
// # Safety
//
// Every read is bounds-checked: malformed input yields an error, never a panic.
//
// # Buffer ownership
//
// A parsed value owns all of its memory. The buffer passed to a Parse function
// may be reused or mutated as soon as it returns.
//
// # Encoding
//
// AppendBinary returns the caller's buffer unchanged on error, never a partial
// encoding of the element that failed; MarshalBinary returns nil. A message is plain
// data: construct it with keyed literals, and do not compare messages with ==.
//
// # Round trips
//
// Everything decoded re-encodes, re-encoding an encoding yields the same octets,
// and input already in the order its spec table defines re-encodes byte for
// byte. Input whose optional elements arrive out of that order is canonicalized
// on the first encode.
//
// # Errors
//
// A parse that returns a non-nil message alongside a non-nil error found only
// syntactically incorrect optional elements, which TS 24.501 §7.7.1 and
// TS 24.301 §7.7.1 require a receiver to treat as not present. [SoftOnly]
// reports that case, and [IEErrors] lists each element that failed.
package nas
