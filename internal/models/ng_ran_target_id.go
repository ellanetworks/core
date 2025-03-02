package models

type NgRanTargetId struct {
	RanNodeId *GlobalRanNodeId `json:"ranNodeId"`
	Tai       *Tai             `json:"tai"`
}
