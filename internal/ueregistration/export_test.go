// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration

import "time"

func (r *Registry) SetRetryIntervalForTest(d time.Duration) {
	r.purgeRetry = d
}
