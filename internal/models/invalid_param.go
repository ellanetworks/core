package models

type InvalidParam struct {
	// Attribute's name encoded as a JSON Pointer, or header's name.
	Param string
	// A human-readable reason, e.g. \"must be a positive integer\".
	Reason string
}
