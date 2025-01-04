// Copyright 2024 Ella Networks

package models

type Profile struct {
	Name string

	UeIpPool        string
	Dns             string
	Mtu             int32
	BitrateUplink   string
	BitrateDownlink string
	Var5qi          int32
	PriorityLevel   int32
}
