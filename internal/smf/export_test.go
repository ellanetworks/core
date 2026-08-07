// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

// LockForTest takes the session's lock and returns its release, so a test in the
// external package can read what the SMF guards. Entry points reach a locked
// session only through sessionFor or sessionBinding, which name the access they
// speak for; a test speaks for none.
func (sc *SMContext) LockForTest() func() {
	sc.mu.Lock()

	return sc.mu.Unlock
}
