// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package configmodels

type SubsListIE struct {
	PlmnID string `json:"plmnID"`
	UeId   string `json:"ueId"`
}
