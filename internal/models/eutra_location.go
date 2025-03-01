package models

import (
	"time"
)

type EutraLocation struct {
	// Tai                      *Tai             `json:"tai" yaml:"tai" bson:"tai"`
	// Ecgi                     *Ecgi            `json:"ecgi" yaml:"ecgi" bson:"ecgi"`
	AgeOfLocationInformation int32      `json:"ageOfLocationInformation,omitempty" yaml:"ageOfLocationInformation" bson:"ageOfLocationInformation"`
	UeLocationTimestamp      *time.Time `json:"ueLocationTimestamp,omitempty" yaml:"ueLocationTimestamp" bson:"ueLocationTimestamp"`
	GeographicalInformation  string     `json:"geographicalInformation,omitempty" yaml:"geographicalInformation" bson:"geographicalInformation"`
	GeodeticInformation      string     `json:"geodeticInformation,omitempty" yaml:"geodeticInformation" bson:"geodeticInformation"`
	// GlobalNgenbId            *GlobalRanNodeId `json:"globalNgenbId,omitempty" yaml:"globalNgenbId" bson:"globalNgenbId"`
}
