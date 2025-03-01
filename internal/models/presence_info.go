package models

type PresenceInfo struct {
	PraId            string
	PresenceState    PresenceState
	TrackingAreaList []Tai
	// EcgiList            []Ecgi            `json:"ecgiList,omitempty" yaml:"ecgiList" bson:"ecgiList" mapstructure:"EcgiList"`
	// NcgiList            []Ncgi            `json:"ncgiList,omitempty" yaml:"ncgiList" bson:"ncgiList" mapstructure:"NcgiList"`
	// GlobalRanNodeIdList []GlobalRanNodeId `json:"globalRanNodeIdList,omitempty" yaml:"globalRanNodeIdList" bson:"globalRanNodeIdList" mapstructure:"GlobalRanNodeIdList"`
}
