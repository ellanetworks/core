package models

type AllowedSnssai struct {
	AllowedSnssai      *Snssai          `json:"allowedSnssai"`
	NsiInformationList []NsiInformation `json:"nsiInformationList,omitempty"`
	MappedHomeSnssai   *Snssai          `json:"mappedHomeSnssai,omitempty"`
}
