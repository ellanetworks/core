package models

type TrafficClassInfo struct {
	// Traffic class name
	Name string `json:"name,omitempty"`

	// QCI/5QI/QFI
	Qci int32 `json:"qci,omitempty"`

	// Traffic class priority
	Arp int32 `json:"arp,omitempty"`

	// Packet Delay Budget
	Pdb int32 `json:"pdb,omitempty"`

	// Packet Error Loss Rate
	Pelr int32 `json:"pelr,omitempty"`
}
