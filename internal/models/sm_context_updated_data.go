package models

type SmContextUpdatedData struct {
	UpCnxState     UpCnxState `json:"upCnxState,omitempty"`
	HoState        HoState    `json:"hoState,omitempty"`
	ReleaseEbiList []int32    `json:"releaseEbiList,omitempty"`
	// AllocatedEbiList []EbiArpMapping  `json:"allocatedEbiList,omitempty"`
	// ModifiedEbiList  []EbiArpMapping  `json:"modifiedEbiList,omitempty"`
	N1SmMsg        *RefToBinaryData `json:"n1SmMsg,omitempty"`
	N2SmInfo       *RefToBinaryData `json:"n2SmInfo,omitempty"`
	N2SmInfoType   N2SmInfoType     `json:"n2SmInfoType,omitempty"`
	EpsBearerSetup []string         `json:"epsBearerSetup,omitempty"`
	DataForwarding bool             `json:"dataForwarding,omitempty"`
}
