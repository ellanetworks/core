// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// RawOctets is an element carried through as bytes: its contents belong to
// another protocol (an EAP packet, RFC 3748) or are a blob the network does not
// interpret. Hex matches how every other raw value in the decoders is rendered.
type RawOctets struct {
	Hex string `json:"hex"`
}

// NewRawOctets renders the element, or nil when it is absent so the field stays
// out of the JSON.
func NewRawOctets(b []byte) *RawOctets {
	if len(b) == 0 {
		return nil
	}

	return &RawOctets{Hex: hex.EncodeToString(b)}
}

// AlgorithmNames names the algorithms a security capability octet advertises,
// as "<prefix><identity>" — EEA0, EIA2, UEA1 and so on.
func AlgorithmNames(prefix string, set nas.AlgorithmSet) []string {
	identities := set.Identities()
	if len(identities) == 0 {
		return nil
	}

	names := make([]string, 0, len(identities))
	for _, n := range identities {
		names = append(names, fmt.Sprintf("%s%d", prefix, n))
	}

	return names
}

// KeySetIdentifier is the NAS key set identifier and the type-of-security-context
// flag that shares its half octet (TS 24.501 §9.11.3.32, TS 24.301 §9.9.3.21).
// They are separate fields: rendering the half octet as one number conflates a
// key set identifier of 1 in a mapped context with one of 9.
type KeySetIdentifier struct {
	Value uint8 `json:"value"`
	// Mapped reports a context mapped from the other generation's, rather than
	// one native to this one.
	Mapped bool `json:"mapped"`
	// NoKey reports the reserved value that stands for no key available.
	NoKey bool `json:"no_key,omitempty"`
}

// NewKeySetIdentifier renders the identifier and its context flag.
func NewKeySetIdentifier(k nas.KeySetIdentifier) KeySetIdentifier {
	return KeySetIdentifier{
		Value:  k.Value,
		Mapped: k.Mapped,
		NoKey:  k.Value == nas.NoKeyAvailable,
	}
}
