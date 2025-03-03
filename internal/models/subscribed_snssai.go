package models

type SubscribedSnssai struct {
	SubscribedSnssai *Snssai `json:"subscribedSnssai" bson:"subscribedSnssai"`

	DefaultIndication bool `json:"defaultIndication,omitempty" bson:"defaultIndication"`
}
