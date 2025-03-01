package models

type RequestTrigger string

// List of RequestTrigger
const (
	RequestTrigger_LOC_CH       RequestTrigger = "LOC_CH"
	RequestTrigger_PRA_CH       RequestTrigger = "PRA_CH"
	RequestTrigger_SERV_AREA_CH RequestTrigger = "SERV_AREA_CH"
	RequestTrigger_RFSP_CH      RequestTrigger = "RFSP_CH"
)
