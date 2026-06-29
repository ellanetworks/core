// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/internal/sctp"

// S1apPPID is the SCTP payload protocol identifier for S1AP (TS 36.412 §7),
// in host order; the inbound check compares against it.
const S1apPPID uint32 = 18

// S1apWirePPID is S1apPPID in the big-endian wire order the socket layer writes
// verbatim (TS 36.412 §7), used when sending.
var S1apWirePPID = sctp.PPIDWireOrder(S1apPPID)

// SCTP stream identifiers for S1AP signalling: stream 0 is reserved for
// non-UE-associated procedures, and UE-associated signalling uses a distinct,
// stable stream (TS 36.412 §7).
const (
	S1apStreamNonUE uint16 = 0
	S1apStreamUE    uint16 = 1
)
