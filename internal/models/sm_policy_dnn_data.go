package models

// Contains the SM policy data for a given DNN (and S-NSSAI).
type SmPolicyDnnData struct {
	Dnn   string
	GbrUl string
	GbrDl string
}
