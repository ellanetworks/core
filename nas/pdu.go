// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

// MaxPDULen is the longest NAS message this library decodes or encodes.
//
// 3GPP sets no length on a NAS message itself, but every container that relays
// one — the NAS message container of TS 24.501 §9.11.3.33, the ESM message
// container of TS 24.301 §9.9.3.15 — states its length in two octets, so a
// longer message could not be carried. Rejecting one at the boundary also caps
// what a single malformed PDU can make a receiver allocate.
const MaxPDULen = 65535

// CheckPDULen reports whether n octets are a NAS message this library will
// handle. Decoding rejects an over-long PDU before reading it, and encoding
// before returning it.
func CheckPDULen(n int) error {
	if n > MaxPDULen {
		return fmt.Errorf("nas: NAS message is %d octets, want at most %d", n, MaxPDULen)
	}

	return nil
}
