// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

type QosData struct {
	QFI    uint8
	Var5qi int32
	Arp    *Arp
}
