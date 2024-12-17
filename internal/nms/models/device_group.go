package models

type Profile struct {
	Name  string   `json:"name"`
	Imsis []string `json:"imsis"`

	Dnn            string `json:"dnn,omitempty"`
	UeIpPool       string `json:"ue-ip-pool,omitempty"`
	DnsPrimary     string `json:"dns-primary,omitempty"`
	DnsSecondary   string `json:"dns-secondary,omitempty"`
	Mtu            int32  `json:"mtu,omitempty"`
	DnnMbrUplink   int64  `json:"dnn-mbr-uplink,omitempty"`
	DnnMbrDownlink int64  `json:"dnn-mbr-downlink,omitempty"`
	BitrateUnit    string `json:"bitrate-unit,omitempty"`
	Qci            int32  `json:"qci,omitempty"`
	Arp            int32  `json:"arp,omitempty"`
	Pdb            int32  `json:"pdb,omitempty"`
	Pelr           int32  `json:"pelr,omitempty"`
}
