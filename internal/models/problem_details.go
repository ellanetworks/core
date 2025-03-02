package models

type ProblemDetails struct {
	// string providing an URI formatted according to IETF RFC 3986.
	Type string
	// A short, human-readable summary of the problem type. It should not change from occurrence to occurrence of the problem.
	Title string
	// The HTTP status code for this occurrence of the problem.
	Status int32
	// A human-readable explanation specific to this occurrence of the problem.
	Detail string
	// string providing an URI formatted according to IETF RFC 3986.
	Instance string
	// A machine-readable application error cause specific to this occurrence of the problem. This IE should be present and provide application-related error information, if available.
	Cause string
	// Description of invalid parameters, for a request rejected due to invalid parameters.
	InvalidParams []InvalidParam
}
