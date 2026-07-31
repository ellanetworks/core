// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package rrctest provides NR-RRC-like message types for conformance testing of
// the UNALIGNED PER variant NR-RRC uses (3GPP TS 38.331 §8.1).
package rrctest

//go:generate sh -c "cd ../../../.. && go run ./cmd/pergen -o per/internal/conformance/rrc/per_gen.go github.com/ellanetworks/core/per/internal/conformance/rrc"

// RRCRelease models the NR-RRC RRCRelease message (TS 38.331 §6.4.2).
type RRCRelease struct {
	RRCTransactionID   int           `per:",range:0..3"`
	CriticalExtensions ReleaseChoice `per:",choice:0"`
}

// ReleaseChoice is the criticalExtensions CHOICE of RRCRelease.
type ReleaseChoice struct {
	RRCRelease *RRCReleaseIEs `per:",choice:0,optional"`
	Late       *bool          `per:",choice:1,optional"`
}

type RRCReleaseIEs struct {
	Deprioritisation *DeprioritisationReq `per:",optional"`
}

type DeprioritisationReq struct {
	Type     int   `per:",range:0..1"` // ENUMERATED { freq, nr }
	Time     int   `per:",range:0..1"` // ENUMERATED { s5, s10 }
	Extended *bool `per:",optional"`
}
