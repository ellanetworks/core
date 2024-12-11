package models

// SliceSiteInfo - give details of the site where this device group is activated
type SliceSiteInfo struct {
	// Unique name per Site.
	SiteName string `json:"site-name,omitempty"`

	Plmn SliceSiteInfoPlmn `json:"plmn,omitempty"`

	GNodeBs []SliceSiteInfoGNodeBs `json:"gNodeBs"`

	// UPF which belong to this slice
	Upf map[string]interface{} `json:"upf,omitempty"`
}
