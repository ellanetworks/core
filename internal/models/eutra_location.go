// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"time"
)

type EutraLocation struct {
	Tai                      *Tai
	Ecgi                     *Ecgi
	AgeOfLocationInformation int32
	UeLocationTimestamp      *time.Time
}
