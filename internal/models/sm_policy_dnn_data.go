package models

// Contains the SM policy data for a given DNN (and S-NSSAI).
type SmPolicyDnnData struct {
	Dnn       string
	GbrUl     string
	GbrDl     string
	Ipv4Index int32
	Ipv6Index int32
	Offline   bool
	Online    bool
}
