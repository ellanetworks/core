package models

type N1N2MsgTxfrErrDetail struct {
	RetryAfter     int32 `json:"retryAfter,omitempty"`
	HighestPrioArp *Arp  `json:"highestPrioArp,omitempty"`
}
