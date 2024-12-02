package models

type SliceSiteInfoGNodeBs struct {
	Name string `json:"name,omitempty"`

	// unique tac per gNB. This should match gNB configuration.
	Tac int32 `json:"tac,omitempty"`
}
