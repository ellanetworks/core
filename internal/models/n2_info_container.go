package models

type N2InfoContainer struct {
	N2InformationClass N2InformationClass `json:"n2InformationClass"`
	SmInfo             *N2SmInformation   `json:"smInfo,omitempty"`
	// RanInfo            *N2RanInformation  `json:"ranInfo,omitempty"`
	// NrppaInfo          *NrppaInformation  `json:"nrppaInfo,omitempty"`
	// PwsInfo            *PwsInformation    `json:"pwsInfo,omitempty"`
}
