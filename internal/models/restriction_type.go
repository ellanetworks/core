package models

type RestrictionType string

// List of RestrictionType
const (
	RestrictionType_ALLOWED_AREAS     RestrictionType = "ALLOWED_AREAS"
	RestrictionType_NOT_ALLOWED_AREAS RestrictionType = "NOT_ALLOWED_AREAS"
)
