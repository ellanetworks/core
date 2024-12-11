package models

type AmPolicyData struct {
	UeId      string   `json:"ueId" yaml:"ueId" bson:"ueId" mapstructure:"UeId"`
	SubscCats []string `json:"subscCats,omitempty" bson:"subscCats"`
}
