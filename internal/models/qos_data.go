// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type QosData struct {
	QFI    uint8
	Var5qi int32
	Arp    *Arp
}
