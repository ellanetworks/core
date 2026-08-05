// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// Ambr is an aggregate maximum bit rate pair. The rates are parsed once, where
// the configured text enters the system, so no consumer re-parses them.
type Ambr struct {
	Uplink   BitRate
	Downlink BitRate
}
