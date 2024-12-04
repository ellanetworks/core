package models

type SliceSiteInfoPlmn struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type SliceSiteInfoGNodeBs struct {
	Name string `json:"name,omitempty"`
	Tac  int32  `json:"tac,omitempty"`
}

type SliceSliceId struct {
	Sst string `json:"sst,omitempty"`
	Sd  string `json:"sd,omitempty"`
}

type SliceSiteInfo struct {
	SiteName string                 `json:"site-name,omitempty"`
	Plmn     SliceSiteInfoPlmn      `json:"plmn,omitempty"`
	GNodeBs  []SliceSiteInfoGNodeBs `json:"gNodeBs"`
	Upf      map[string]interface{} `json:"upf,omitempty"`
}

type SliceApplicationFilteringRules struct {
	RuleName       string            `json:"rule-name,omitempty"`
	Priority       int32             `json:"priority,omitempty"`
	Action         string            `json:"action,omitempty"`
	Endpoint       string            `json:"endpoint,omitempty"`
	Protocol       int32             `json:"protocol,omitempty"`
	StartPort      int32             `json:"dest-port-start,omitempty"`
	EndPort        int32             `json:"dest-port-end,omitempty"`
	AppMbrUplink   int32             `json:"app-mbr-uplink,omitempty"`
	AppMbrDownlink int32             `json:"app-mbr-downlink,omitempty"`
	BitrateUnit    string            `json:"bitrate-unit,omitempty"`
	TrafficClass   *TrafficClassInfo `json:"traffic-class,omitempty"`
	RuleTrigger    string            `json:"rule-trigger,omitempty"`
}

type Slice struct {
	SliceName                 string                           `json:"slice-name,omitempty"`
	SliceId                   SliceSliceId                     `json:"slice-id,omitempty"`
	SiteDeviceGroup           []string                         `json:"site-device-group"`
	SiteInfo                  SliceSiteInfo                    `json:"site-info,omitempty"`
	ApplicationFilteringRules []SliceApplicationFilteringRules `json:"application-filtering-rules,omitempty"`
}
