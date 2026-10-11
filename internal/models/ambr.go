// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// Ambr is an aggregate maximum bit rate pair. The rates are parsed once, where
// the configured text enters the system, so no consumer re-parses them.
type Ambr struct {
	Uplink   BitRate
	Downlink BitRate
}

func (a Ambr) Equal(o Ambr) bool {
	return a.Uplink.Equal(o.Uplink) && a.Downlink.Equal(o.Downlink)
}

func (a Ambr) Max(o Ambr) Ambr {
	return Ambr{
		Uplink:   BitRateFromBps(max(a.Uplink.Bps(), o.Uplink.Bps())),
		Downlink: BitRateFromBps(max(a.Downlink.Bps(), o.Downlink.Bps())),
	}
}
