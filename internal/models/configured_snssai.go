package models

type ConfiguredSnssai struct {
	ConfiguredSnssai *Snssai `json:"configuredSnssai"`
	MappedHomeSnssai *Snssai `json:"mappedHomeSnssai,omitempty"`
}
