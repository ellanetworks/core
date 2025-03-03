package models

type NsiInformation struct {
	NrfId string `json:"nrfId" yaml:"nrfId"`
	NsiId string `json:"nsiId,omitempty" yaml:"nsiId"`
}
