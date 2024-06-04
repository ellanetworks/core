// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package configmodels

import (
	"github.com/omec-project/openapi/models"
)

const (
	Post_op = iota
	Put_op
	Delete_op
)

const (
	Device_group = iota
	Network_slice
	Sub_data
)

type ConfigMessage struct {
	DevGroup     *DeviceGroups
	Slice        *Slice
	AuthSubData  *models.AuthenticationSubscription
	DevGroupName string
	SliceName    string
	Imsi         string
	MsgType      int
	MsgMethod    int
}

// Slice + attached device group
type SliceConfigSnapshot struct {
	SliceMsg *Slice
	DevGroup []*DeviceGroups
}

// DevGroup + slice name
type DevGroupConfigSnapshot struct {
	DevGroup  *DeviceGroups
	SliceName string
}
