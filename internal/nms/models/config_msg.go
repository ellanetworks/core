package models

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
	Inventory
)

type ConfigMessage struct {
	DevGroup     *DeviceGroups
	Slice        *Slice
	AuthSubData  *models.AuthenticationSubscription
	Gnb          *Gnb
	DevGroupName string
	SliceName    string
	Imsi         string
	GnbName      string
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
