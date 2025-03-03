package models

type AllowedSnssai struct {
	AllowedSnssai      *Snssai
	NsiInformationList []NsiInformation
	MappedHomeSnssai   *Snssai
}
