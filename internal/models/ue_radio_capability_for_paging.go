package models

type UERadioCapabilityForPaging struct {
	NR    string // OCTET string
	EUTRA string // OCTET string
}

// TS 38.413 9.3.1.71
type RecommendedCell struct {
	NgRanCGI         NGRANCGI
	TimeStayedInCell *int64
}

type NGRANCGI struct {
	Present  int32
	NRCGI    *Ncgi
	EUTRACGI *Ecgi
}

// TS 38.413 9.3.1.100
type InfoOnRecommendedCellsAndRanNodesForPaging struct {
	RecommendedCells []RecommendedCell
}

const (
	NgRanCgiPresentNRCGI    int32 = 0
	NgRanCgiPresentEUTRACGI int32 = 1
)
