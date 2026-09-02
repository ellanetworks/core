// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

// b2u renders a decoded boolean flag as the 0/1 the observability JSON uses.
func b2u(b bool) uint8 {
	if b {
		return 1
	}

	return 0
}
