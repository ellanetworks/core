// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package rrctest provides simplified NR-RRC-like message types for conformance
// testing of UNALIGNED PER encoding. NR-RRC uses BASIC-PER Unaligned variant
// per 3GPP TS 38.331 §8.1.
package rrctest

//go:generate go run github.com/ellanetworks/core/cmd/pergen

// RRCRelease is a simplified version of the NR-RRC RRCRelease message.
// TS 38.331 §6.4.2. It contains:
//   - rrc-TransactionIdentifier: INTEGER (0..3)
//   - criticalExtensions: CHOICE { rrcRelease, late }
type RRCRelease struct {
	RRCTransactionID   int             `per:",range:0..3"`
	CriticalExtensions ReleaseChoice   `per:",choice:0,optional"`
}

// ReleaseChoice models the criticalExtensions CHOICE of RRCRelease.
type ReleaseChoice struct {
	RRCRelease *RRCReleaseIEs `per:",choice:0,optional"`
	Late       *bool          `per:",choice:1,optional"`
}

// RRCReleaseIEs contains the actual RRCRelease content.
type RRCReleaseIEs struct {
	Deprioritisation *DeprioritisationReq `per:",optional"`
}

// DeprioritisationReq is a simplified deprioritisation request.
type DeprioritisationReq struct {
	Type    int  `per:",range:0..1"`     // ENUMERATED { freq, nr }
	Time    int  `per:",range:0..1"`     // ENUMERATED { s5, s10 }
	Extended *bool `per:",optional"`
}
