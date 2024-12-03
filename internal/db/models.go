package db

type TrafficClassInfo struct {
	Name string
	Qci  int32
	Arp  int32
	Pdb  int32
	Pelr int32
}

type DeviceGroupsIpDomainExpandedUeDnnQos struct {
	DnnMbrUplink   int64
	DnnMbrDownlink int64
	BitrateUnit    string
	TrafficClass   *TrafficClassInfo
}

type DeviceGroupsIpDomainExpanded struct {
	Dnn          string
	UeIpPool     string
	DnsPrimary   string
	DnsSecondary string
	Mtu          int32
	UeDnnQos     *DeviceGroupsIpDomainExpandedUeDnnQos
}

type DeviceGroup struct {
	DeviceGroupName  string
	Imsis            []string
	SiteInfo         string
	IpDomainName     string
	IpDomainExpanded DeviceGroupsIpDomainExpanded
}
