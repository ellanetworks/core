package models

type Subscriber struct {
	UeId string `json:"ueId"`

	// Network Configuration
	PlmnID string `json:"plmnID"`
	Sst    int32  `json:"sst" yaml:"sst" bson:"sst" mapstructure:"Sst"`
	Sd     string `json:"sd,omitempty" yaml:"sd" bson:"sd" mapstructure:"Sd"`
	Dnn    string `json:"dnn" yaml:"dnn" bson:"dnn" mapstructure:"Dnn"`

	// Security Configuration
	SequenceNumber    string `json:"sequenceNumber" bson:"sequenceNumber"`
	PermanentKeyValue string `json:"permanentKeyValue" bson:"permanentKeyValue"`
	OpcValue          string `json:"opcValue" bson:"opcValue"`

	// QoS Configuration
	BitRateUplink   string `json:"uplink" yaml:"uplink" bson:"uplink" mapstructure:"Uplink"`
	BitRateDownlink string `json:"downlink" yaml:"downlink" bson:"downlink" mapstructure:"Downlink"`
	Var5qi          int32  `json:"5qi" yaml:"5qi" bson:"5qi" mapstructure:"Var5qi"`
	PriorityLevel   int32  `json:"priorityLevel" yaml:"priorityLevel" bson:"priorityLevel" mapstructure:"PriorityLevel"`
}
