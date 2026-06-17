// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package models

type QosData struct {
	QFI    uint8
	Var5qi int32
	Arp    *Arp
}
