package models

type Profile struct {
	Name  string
	Imsis []string

	Dnn             string
	UeIpPool        string
	DnsPrimary      string
	DnsSecondary    string
	Mtu             int32
	BitrateUplink   int64
	BitrateDownlink int64
	BitrateUnit     string
	Var5qi          int32
	Arp             int32
	Pdb             int32
	Pelr            int32
}
