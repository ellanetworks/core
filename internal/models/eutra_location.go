package models

import (
	"time"
)

type EutraLocation struct {
	Tai                      *Tai
	Ecgi                     *Ecgi
	AgeOfLocationInformation int32
	UeLocationTimestamp      *time.Time
	GeographicalInformation  string
	GeodeticInformation      string
	GlobalNgenbId            *GlobalRanNodeId
}
