// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type PlmnSupportItem struct {
	PlmnID     PlmnID
	SNssaiList []Snssai
}
