package models

type SmContextUpdatedData struct {
	UpCnxState     UpCnxState
	HoState        HoState
	ReleaseEbiList []int32
	N1SmMsg        *RefToBinaryData
	N2SmInfo       *RefToBinaryData
	N2SmInfoType   N2SmInfoType
	EpsBearerSetup []string
	DataForwarding bool
}
