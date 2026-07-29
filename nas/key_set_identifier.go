// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

// KeySetIdentifier is a NAS key set identifier: which security context a message
// refers to, and whether that context is native to the generation carrying it or
// mapped from the other one (TS 24.501 §9.11.3.32, TS 24.301 §9.9.3.21). Both
// generations code it identically, in four bits.
//
// The zero value identifies native key set 0. A UE holding no context sends
// NoKeyAvailable, which Available reports.
type KeySetIdentifier struct {
	// Value is the key set identifier, 0 to 6, or NoKeyAvailable.
	Value uint8
	// Mapped is the type of security context flag: false native, true mapped
	// from the other generation's context.
	Mapped bool
}

// NoKeyAvailable is the key set identifier value that stands for no key
// (TS 24.501 §9.11.3.32, TS 24.301 §9.9.3.21).
const NoKeyAvailable uint8 = 0x07

// NoKeySet is the identifier a sender with no security context uses.
var NoKeySet = KeySetIdentifier{Value: NoKeyAvailable}

// ParseKeySetIdentifier decodes a key set identifier from the four bits it
// occupies, ignoring the other four bits of the octet.
func ParseKeySetIdentifier(halfOctet uint8) KeySetIdentifier {
	return KeySetIdentifier{Value: halfOctet & 0x07, Mapped: halfOctet&0x08 != 0}
}

// HalfOctet encodes the identifier into the low four bits of an octet, which the
// caller shifts into whichever half the message puts it in.
func (k KeySetIdentifier) HalfOctet() uint8 {
	o := k.Value & 0x07
	if k.Mapped {
		o |= 0x08
	}

	return o
}

// Available reports whether the identifier names a key set at all.
func (k KeySetIdentifier) Available() bool { return k.Value&0x07 != NoKeyAvailable }

func (k KeySetIdentifier) String() string {
	if !k.Available() {
		return "no key available"
	}

	if k.Mapped {
		return fmt.Sprintf("mapped %d", k.Value&0x07)
	}

	return fmt.Sprintf("native %d", k.Value&0x07)
}
