package models

type AuthenticationInfo struct {
	SupiOrSuci            string
	ServingNetworkName    string
	ResynchronizationInfo *ResynchronizationInfo
	// TraceData             *TraceData             `json:"traceData,omitempty" yaml:"traceData" bson:"traceData"`
}
