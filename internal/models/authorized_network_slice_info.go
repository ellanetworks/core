package models

type AuthorizedNetworkSliceInfo struct {
	AllowedNssaiList    []AllowedNssai
	ConfiguredNssai     []ConfiguredSnssai
	TargetAmfSet        string
	CandidateAmfList    []string
	RejectedNssaiInPlmn []Snssai
	RejectedNssaiInTa   []Snssai
	NsiInformation      *NsiInformation
	SupportedFeatures   string
	NrfAmfSet           string
}
