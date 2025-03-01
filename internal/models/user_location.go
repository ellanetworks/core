package models

type UserLocation struct {
	EutraLocation *EutraLocation
	NrLocation    *NrLocation
	// N3gaLocation  *N3gaLocation  `json:"n3gaLocation,omitempty" yaml:"n3gaLocation" bson:"n3gaLocation"`
}
