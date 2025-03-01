package models

type PolicyAssociation struct {
	Request *PolicyAssociationRequest `json:"request,omitempty" yaml:"request" bson:"request" mapstructure:"Request"`
	// Request Triggers that the PCF subscribes. Only values \"LOC_CH\" and \"PRA_CH\" are permitted.
	Triggers    []RequestTrigger        `json:"triggers,omitempty" yaml:"triggers" bson:"triggers" mapstructure:"Triggers"`
	ServAreaRes *ServiceAreaRestriction `json:"servAreaRes,omitempty" yaml:"servAreaRes" bson:"servAreaRes" mapstructure:"ServAreaRes"`
	Rfsp        int32                   `json:"rfsp,omitempty" yaml:"rfsp" bson:"rfsp" mapstructure:"Rfsp"`
	Pras        map[string]PresenceInfo `json:"pras,omitempty" yaml:"pras" bson:"pras" mapstructure:"Pras"`
	SuppFeat    string                  `json:"suppFeat" yaml:"suppFeat" bson:"suppFeat" mapstructure:"SuppFeat"`
}
