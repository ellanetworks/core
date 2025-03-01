package models

type Nssai struct {
	SupportedFeatures   string
	DefaultSingleNssais []Snssai
	SingleNssais        []Snssai
}
