// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb

import "time"

// Timeouts for the NG-eNB lifecycle procedures, named as in the gnb and s1enb
// scenarios so every radio's scenarios read alike.
const (
	registrationTimeout = 8 * time.Second
	releaseTimeout      = 2 * time.Second
)
