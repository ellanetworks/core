// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package eps implements the EPS NAS codec of TS 24.301 (4G): EMM/ESM message
// headers, the security-protected message wrapper, and the message and
// information-element catalog. It builds on github.com/ellanetworks/core/nas
// for octet framing and the integrity/cipher algorithms.
//
// It mirrors [github.com/ellanetworks/core/nas/fgs], the 5GS codec: the same
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
// syntactically incorrect optional elements, which TS 24.301 §7.7.1 requires a
// receiver to treat as not present. [nas.SoftOnly] reports that case, and
// [nas.IEErrors] lists each element that failed.
package eps
