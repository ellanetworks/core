package models

type TrafficClassInfo struct {
	Name string `json:"name,omitempty"`
	Qci  int32  `json:"qci,omitempty"`
	Arp  int32  `json:"arp,omitempty"`
	Pdb  int32  `json:"pdb,omitempty"`
	Pelr int32  `json:"pelr,omitempty"`
}

type DeviceGroupsIpDomainExpandedUeDnnQos struct {
	DnnMbrUplink   int64             `json:"dnn-mbr-uplink,omitempty"`
	DnnMbrDownlink int64             `json:"dnn-mbr-downlink,omitempty"`
	BitrateUnit    string            `json:"bitrate-unit,omitempty"`
	TrafficClass   *TrafficClassInfo `json:"traffic-class,omitempty"`
}

type DeviceGroupsIpDomainExpanded struct {
	Dnn          string                                `json:"dnn,omitempty"`
	UeIpPool     string                                `json:"ue-ip-pool,omitempty"`
	DnsPrimary   string                                `json:"dns-primary,omitempty"`
	DnsSecondary string                                `json:"dns-secondary,omitempty"`
	Mtu          int32                                 `json:"mtu,omitempty"`
	UeDnnQos     *DeviceGroupsIpDomainExpandedUeDnnQos `json:"ue-dnn-qos,omitempty"`
}

type DeviceGroup struct {
	DeviceGroupName  string                       `json:"group-name"`
	Imsis            []string                     `json:"imsis"`
	SiteInfo         string                       `json:"site-info,omitempty"`
	IpDomainName     string                       `json:"ip-domain-name,omitempty"`
	IpDomainExpanded DeviceGroupsIpDomainExpanded `json:"ip-domain-expanded,omitempty"`
}
