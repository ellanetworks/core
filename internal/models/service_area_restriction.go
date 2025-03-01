package models

type ServiceAreaRestriction struct {
	RestrictionType RestrictionType
	Areas           []Area
	MaxNumOfTAs     int32
}
