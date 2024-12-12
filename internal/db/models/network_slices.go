package models

type GNodeB struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type NetworkSlice struct {
	Name         string                 `json:"name,omitempty"`
	Sst          string                 `json:"sst,omitempty"`
	Sd           string                 `json:"sd,omitempty"`
	DeviceGroups []string               `json:"device-group"`
	Mcc          string                 `json:"mcc,omitempty"`
	Mnc          string                 `json:"mnc,omitempty"`
	GNodeBs      []GNodeB               `json:"gNodeBs"`
	Upf          map[string]interface{} `json:"upf,omitempty"`
}
