package models

type Tai struct {
	PlmnId *PlmnId `json:"plmnId" yaml:"plmnId" bson:"plmnId" mapstructure:"PlmnId"`
	Tac    string  `json:"tac" yaml:"tac" bson:"tac" mapstructure:"Tac"`
}
