// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

type PlmnSupportItem struct {
	PlmnID     PlmnID
	SNssaiList []Snssai
}
