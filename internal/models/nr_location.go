package models

import (
	"time"
)

type NrLocation struct {
	Tai                      *Tai
	Ncgi                     *Ncgi
	AgeOfLocationInformation int32
	UeLocationTimestamp      *time.Time
	GeographicalInformation  string
	GeodeticInformation      string
	GlobalGnbId              *GlobalRanNodeId
}
