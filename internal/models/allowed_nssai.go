package models

type AllowedNssai struct {
	AllowedSnssaiList []AllowedSnssai `json:"allowedSnssaiList"`
	AccessType        AccessType      `json:"accessType"`
}
