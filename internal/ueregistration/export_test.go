// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration

import "time"

func (r *Registry) SetIntervalForTest(d time.Duration) {
	r.interval = d
}
