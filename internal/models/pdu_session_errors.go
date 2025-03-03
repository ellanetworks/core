package models

import (
	"net/http"
)

var N1SmError = ProblemDetails{
	Title:  "Invalid N1 Message",
	Status: http.StatusForbidden,
	Detail: "N1 Message Error",
	Cause:  "N1_SM_ERROR",
}
