package models

type SliceSiteInfoGNodeBs struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type SliceSiteInfo struct {
	SiteName string                 `json:"site-name,omitempty"`
	Plmn     SliceSiteInfoPlmn      `json:"plmn,omitempty"`
	GNodeBs  []SliceSiteInfoGNodeBs `json:"gNodeBs"`
	Upf      map[string]interface{} `json:"upf,omitempty"`
}

type Slice struct {
	SliceName                 string                           `json:"slice-name,omitempty"`
	SliceId                   SliceSliceId                     `json:"slice-id,omitempty"`
	SiteDeviceGroup           []string                         `json:"site-device-group"`
	SiteInfo                  SliceSiteInfo                    `json:"site-info,omitempty"`
	ApplicationFilteringRules []SliceApplicationFilteringRules `json:"application-filtering-rules,omitempty"`
}
