package models

type SubscribedDefaultQos struct {
	Var5qi        int32 `json:"5qi" yaml:"5qi" bson:"5qi" mapstructure:"Var5qi"`
	Arp           *Arp  `json:"arp" yaml:"arp" bson:"arp" mapstructure:"Arp"`
	PriorityLevel int32 `json:"priorityLevel,omitempty" yaml:"priorityLevel" bson:"priorityLevel" mapstructure:"PriorityLevel"`
}
