// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nastest builds NAS octet strings for tests: well-formed ones, and the
// malformed ones a decoder has to survive. It is the adversarial-construction
// half of the codec's test surface — nothing here validates anything, since that
// is the job of the decoder under test.
//
// [Builder] appends information elements in the framings TS 24.007 defines, which
// both generations share, and the header-seeded constructors start one at the
// message header of the generation under test: [BuildGMM] and [BuildGSM] for 5GS
// (TS 24.501), [BuildEMM] and [BuildESM] for EPS (TS 24.301). The Raw variants
// take the header octets themselves, for a message whose protocol discriminator
// or type is itself the thing being corrupted.
//
//	pdu := nastest.BuildGMM(fgs.MsgRegistrationRequest).
//		U8(0x01).            // ngKSI and registration type
//		LVE(mobileIdentity). // mandatory 5GS mobile identity
//		TLV(0x2E, capability).
//		Bytes()
//
// To build a message that should not decode, declare a length that does not match
// the value (LVn, LVEn, TLVn, TLVEn), append an IEI the message does not define,
// repeat an element, or Truncate the result mid-element.
//
// This package is exported because consumers outside the module use it, and it is
// exempt from the compatibility promise the codec packages keep: its surface grows
// with every new negative test.
package nastest
