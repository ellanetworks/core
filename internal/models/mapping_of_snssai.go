package models

type MappingOfSnssai struct {
	ServingSnssai *Snssai `json:"servingSnssai" bson:"servingSnssai" yaml:"servingSnssai"`

	HomeSnssai *Snssai `json:"homeSnssai" bson:"homeSnssai" yaml:"homeSnssai"`
}
