// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "encoding/hex"

// NASMessageContainer is the complete initial NAS message an element carries
// (TS 24.501 §9.11.3.33). In SECURITY MODE COMPLETE the whole message is already
// ciphered, so the container is plaintext and decodes; in a cleartext
// REGISTRATION REQUEST the container itself is ciphered and reports as such.
// The shape matches the other embedded PDUs so the raw bytes stay available.
type NASMessageContainer struct {
	Protocol string      `json:"protocol"`
	RawHex   string      `json:"raw_hex"`
	Decoded  *NASMessage `json:"decoded,omitempty"`
}

// nasMessageContainer decodes the element's contents. Recursion terminates
// because a container is one element of the message holding it and so is always
// strictly shorter than it.
func nasMessageContainer(b []byte) *NASMessageContainer {
	if len(b) == 0 {
		return nil
	}

	return &NASMessageContainer{
		Protocol: "NAS",
		RawHex:   hex.EncodeToString(b),
		Decoded:  DecodeNASMessage(b),
	}
}
