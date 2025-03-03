package models

type SeafData struct {
	NgKsi                *NgKsi  `json:"ngKsi"`
	KeyAmf               *KeyAmf `json:"keyAmf"`
	Nh                   string  `json:"nh,omitempty"`
	Ncc                  int32   `json:"ncc,omitempty"`
	KeyAmfChangeInd      bool    `json:"keyAmfChangeInd,omitempty"`
	KeyAmfHDerivationInd bool    `json:"keyAmfHDerivationInd,omitempty"`
}
