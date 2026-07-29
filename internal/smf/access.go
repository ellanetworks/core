// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

// AccessType is the radio access a session is established over. As the combined
// SMF+PGW-C (TS 23.501), the SMF keys its 4G/5G differences off it.
type AccessType uint8

const (
	Access5G AccessType = iota // N3 user plane; 5GSM NAS terminated in the SMF
	Access4G                   // S1-U user plane; ESM owned by the MME (PGW-C role)
)

// IsEPS reports whether the session is a 4G EPS bearer (PGW-C role).
func (sc *SMContext) IsEPS() bool { return sc.Access == Access4G }

// usesPSC reports whether the user-plane GTP-U carries the PDU Session Container
// (and thus the QFI). 5G N3/N9 do; 4G S1-U does not. TS 23.501, TS 38.415.
func (a AccessType) usesPSC() bool { return a == Access5G }

// keyID maps a wire session identity into the SMF's converged id space. A 5G
// session keeps its UE-assigned PDU session identity (1..15, TS 24.007
// §11.2.3.1b); a 4G PDN connection is numbered 64+EBI, inside the
// core-network-allocated PduSessionId range 64..95 that is only visible in the
// core network (TS 29.571 §5.2.2). The two accesses therefore occupy disjoint
// ids, and a lookup keyed on one access cannot resolve a session of the other.
func (a AccessType) keyID(id uint8) uint8 {
	if a == Access4G {
		return 64 + id
	}

	return id
}

// keyID returns the session's id in the converged id space; it is the id the
// session is indexed and its IP leases are keyed by.
func (sc *SMContext) keyID() uint8 { return sc.Access.keyID(sc.PDUSessionID) }
