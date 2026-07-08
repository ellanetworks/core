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
