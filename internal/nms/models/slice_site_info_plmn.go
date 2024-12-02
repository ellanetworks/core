package models

// SliceSiteInfoPlmn - Fixed supported plmn at the site.
type SliceSiteInfoPlmn struct {
	Mcc string `json:"mcc,omitempty"`

	Mnc string `json:"mnc,omitempty"`
}
