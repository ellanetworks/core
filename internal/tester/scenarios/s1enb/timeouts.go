// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import "time"

// Timeouts for the eNB lifecycle procedures. The gnb scenarios name theirs the
// same way, so a 4G and a 5G scenario read alike; only the procedure names
// differ, because EPS attaches where 5GS registers.
const (
	attachTimeout  = 15 * time.Second
	releaseTimeout = 10 * time.Second
)
