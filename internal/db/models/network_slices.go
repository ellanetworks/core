package models

type UPF struct {
	Name string `json:"name,omitempty"`
	Port string `json:"port,omitempty"`
}

type NetworkSlice struct {
	Name         string   `json:"name,omitempty"`
	Sst          string   `json:"sst,omitempty"`
	Sd           string   `json:"sd,omitempty"`
	DeviceGroups []string `json:"device-groups"`
	Mcc          string   `json:"mcc,omitempty"`
	Mnc          string   `json:"mnc,omitempty"`
	GNodeBs      []Radio  `json:"gNodeBs"`
	Upf          UPF      `json:"upf,omitempty"`
}
