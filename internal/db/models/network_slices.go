package models

type SliceSiteInfoGNodeBs struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type Slice struct {
	Name            string                 `json:"name,omitempty"`
	Sst             string                 `json:"sst,omitempty"`
	Sd              string                 `json:"sd,omitempty"`
	SiteDeviceGroup []string               `json:"site-device-group"`
	Mcc             string                 `json:"mcc,omitempty"`
	Mnc             string                 `json:"mnc,omitempty"`
	GNodeBs         []SliceSiteInfoGNodeBs `json:"gNodeBs"`
	Upf             map[string]interface{} `json:"upf,omitempty"`
}
