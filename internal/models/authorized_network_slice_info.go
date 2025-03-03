package models

type AuthorizedNetworkSliceInfo struct {
	AllowedNssaiList    []AllowedNssai
	RejectedNssaiInPlmn []Snssai
	RejectedNssaiInTa   []Snssai
}
