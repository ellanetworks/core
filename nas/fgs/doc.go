// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package fgs implements the 5GS NAS codec of TS 24.501 (5G): 5GMM/5GSM message
// headers, the security-protected message wrapper, and the message and
// information-element catalog. It builds on github.com/ellanetworks/core/nas
// for octet framing and the integrity/cipher algorithms.
//
// It mirrors [github.com/ellanetworks/core/nas/eps], the EPS codec: the same
// concept carries the same type and field names in both, except where 3GPP
// defines a different structure.
//
// # Buffer ownership
//
// A parsed message owns all of its memory. The buffer passed to a Parse
// function may be reused or mutated as soon as it returns.
//
// # Encoding
//
// AppendBinary returns the caller's buffer unchanged on error, never a partial
// encoding of the message; MarshalBinary returns nil. Construct a message with keyed
// literals, and do not compare messages with ==.
//
// # Errors
//
// A parse that returns a non-nil message alongside a non-nil error found only
// syntactically incorrect optional elements, which TS 24.501 §7.7.1 requires a
// receiver to treat as not present. [nas.SoftOnly] reports that case, and
// [nas.IEErrors] lists each element that failed.
package fgs
