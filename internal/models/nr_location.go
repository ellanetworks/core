// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"time"
)

type NrLocation struct {
	Tai                      *Tai
	Ncgi                     *Ncgi
	AgeOfLocationInformation int32
	UeLocationTimestamp      *time.Time
}
