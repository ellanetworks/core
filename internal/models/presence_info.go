package models

type PresenceInfo struct {
	PraId            string `json:"praId,omitempty" yaml:"praId" bson:"praId" mapstructure:"PraId"`
	PresenceState    PresenceState
	TrackingAreaList []Tai `json:"trackingAreaList,omitempty" yaml:"trackingAreaList" bson:"trackingAreaList" mapstructure:"TrackingAreaList"`
	// EcgiList            []Ecgi            `json:"ecgiList,omitempty" yaml:"ecgiList" bson:"ecgiList" mapstructure:"EcgiList"`
	// NcgiList            []Ncgi            `json:"ncgiList,omitempty" yaml:"ncgiList" bson:"ncgiList" mapstructure:"NcgiList"`
	// GlobalRanNodeIdList []GlobalRanNodeId `json:"globalRanNodeIdList,omitempty" yaml:"globalRanNodeIdList" bson:"globalRanNodeIdList" mapstructure:"GlobalRanNodeIdList"`
}
