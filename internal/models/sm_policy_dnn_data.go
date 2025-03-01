package models

// Contains the SM policy data for a given DNN (and S-NSSAI).
type SmPolicyDnnData struct {
	Dnn                 string   `json:"dnn" bson:"dnn"`
	AllowedServices     []string `json:"allowedServices,omitempty" bson:"allowedServices"`
	SubscCats           []string `json:"subscCats,omitempty" bson:"subscCats"`
	GbrUl               string   `json:"gbrUl,omitempty" bson:"gbrUl"`
	GbrDl               string   `json:"gbrDl,omitempty" bson:"gbrDl"`
	AdcSupport          bool     `json:"adcSupport,omitempty" bson:"adcSupport"`
	SubscSpendingLimits bool     `json:"subscSpendingLimits,omitempty" bson:"subscSpendingLimits"`
	Ipv4Index           int32    `json:"ipv4Index,omitempty" bson:"ipv4Index"`
	Ipv6Index           int32    `json:"ipv6Index,omitempty" bson:"ipv6Index"`
	Offline             bool     `json:"offline,omitempty" bson:"offline"`
	Online              bool     `json:"online,omitempty" bson:"online"`
	// ChfInfo             *ChargingInformation              `json:"chfInfo,omitempty" bson:"chfInfo"`
	// RefUmDataLimitIds   map[string]LimitIdToMonitoringKey `json:"refUmDataLimitIds,omitempty" bson:"refUmDataLimitIds"`
	MpsPriority       bool  `json:"mpsPriority,omitempty" bson:"mpsPriority"`
	ImsSignallingPrio bool  `json:"imsSignallingPrio,omitempty" bson:"imsSignallingPrio"`
	MpsPriorityLevel  int32 `json:"mpsPriorityLevel,omitempty" bson:"mpsPriorityLevel"`
}
