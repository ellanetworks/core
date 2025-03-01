package models

type PolicyUpdate struct {
	ResourceUri string `json:"resourceUri" yaml:"resourceUri" bson:"resourceUri" mapstructure:"ResourceUri"`
	// Request Triggers that the PCF subscribes. Only values \"LOC_CH\" and \"PRA_CH\" are permitted.
	Triggers    []RequestTrigger        `json:"triggers,omitempty" yaml:"triggers" bson:"triggers" mapstructure:"Triggers"`
	ServAreaRes *ServiceAreaRestriction `json:"servAreaRes,omitempty" yaml:"servAreaRes" bson:"servAreaRes" mapstructure:"ServAreaRes"`
	Rfsp        int32                   `json:"rfsp,omitempty" yaml:"rfsp" bson:"rfsp" mapstructure:"Rfsp"`
	// Map of PRA information.
	// Pras map[string]PresenceInfoRm `json:"pras,omitempty" yaml:"pras" bson:"pras" mapstructure:"Pras"`
}
