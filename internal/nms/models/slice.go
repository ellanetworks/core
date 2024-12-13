package models

type Slice struct {
	SliceName string `json:"slice-name,omitempty"`

	SliceId SliceSliceId `json:"slice-id,omitempty"`

	SiteDeviceGroup []string `json:"site-device-group"`

	SiteInfo SliceSiteInfo `json:"site-info,omitempty"`

	ApplicationFilteringRules []SliceApplicationFilteringRules `json:"application-filtering-rules,omitempty"`
}
