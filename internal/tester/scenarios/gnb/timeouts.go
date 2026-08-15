// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import "time"

// Timeouts for the gNB lifecycle procedures. The s1enb scenarios name theirs
// the same way, so a 4G and a 5G scenario read alike; only the procedure names
// differ, because 5GS registers where EPS attaches.
const (
	registrationTimeout = 8 * time.Second
	releaseTimeout      = 2 * time.Second
)
