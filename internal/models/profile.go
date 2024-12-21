package models

type Profile struct {
	Name string

	UeIpPool        string
	DnsPrimary      string
	DnsSecondary    string
	Mtu             int32
	BitrateUplink   string
	BitrateDownlink string
	Var5qi          int32
	PriorityLevel   int32
}
