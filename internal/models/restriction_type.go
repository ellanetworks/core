package models

type RestrictionType string

const (
	RestrictionTypeAllowedAreas    RestrictionType = "ALLOWED_AREAS"
	RestrictionTypeNotAllowedAreas RestrictionType = "NOT_ALLOWED_AREAS"
)
