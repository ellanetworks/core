// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import "fmt"

// AccessType is the radio access a session is established over. As the combined
// SMF+PGW-C (TS 23.501), the SMF keys its 4G/5G differences off it.
type AccessType uint8

const (
	Access5G AccessType = iota // N3 user plane; 5GSM NAS terminated in the SMF
	Access4G                   // S1-U user plane; ESM owned by the MME (PGW-C role)
)

// IsEPS reports whether the session is a 4G EPS bearer (PGW-C role).
func (sc *SMContext) IsEPS() bool { return sc.Access == Access4G }

// servedBy reports whether the session is still on the access an entry point
// speaks for. A transfer leaves the source access holding a live Ref until its
// target binds, so a request arriving on the access the session left must not
// act on it. Caller must not hold sc.Mutex.
func (sc *SMContext) servedBy(access AccessType) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Access != access {
		return fmt.Errorf("session %q is on %s", sc.Ref, sc.Access)
	}

	return nil
}

func (a AccessType) String() string {
	if a == Access4G {
		return "EPS"
	}

	return "5GS"
}

// usesPSC reports whether the user-plane GTP-U carries the PDU Session Container
// (and thus the QFI). 5G N3/N9 do; 4G S1-U does not. TS 23.501, TS 38.415.
func (a AccessType) usesPSC() bool { return a == Access5G }
