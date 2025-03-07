package models

type RequestTrigger string

const (
	RequestTriggerLocCh      RequestTrigger = "LOC_CH"
	RequestTriggerPraCh      RequestTrigger = "PRA_CH"
	RequestTriggerServAreaCh RequestTrigger = "SERV_AREA_CH"
	RequestTriggerRfspCh     RequestTrigger = "RFSP_CH"
)
