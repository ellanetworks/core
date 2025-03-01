package models

type UserLocation struct {
	EutraLocation *EutraLocation `json:"eutraLocation,omitempty" yaml:"eutraLocation" bson:"eutraLocation"`
	NrLocation    *NrLocation    `json:"nrLocation,omitempty" yaml:"nrLocation" bson:"nrLocation"`
	// N3gaLocation  *N3gaLocation  `json:"n3gaLocation,omitempty" yaml:"n3gaLocation" bson:"n3gaLocation"`
}
