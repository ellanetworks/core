package fsm

func (e SmEvent) String() string {
	switch e {
	case SmEventPduSessCreate:
		return "SmEventPduSessCreate"
	case SmEventPduSessModify:
		return "SmEventPduSessModify"
	case SmEventPduSessRelease:
		return "SmEventPduSessRelease"
	case SmEventPfcpSessCreate:
		return "SmEventPfcpSessCreate"
	case SmEventPfcpSessModify:
		return "SmEventPfcpSessModify"
	case SmEventPfcpSessRelease:
		return "SmEventPfcpSessRelease"
	case SmEventPduSessN1N2Transfer:
		return "SmEventPduSessN1N2Transfer"
	case SmEventPfcpSessCreateFailure:
		return "SmEventPfcpSessCreateFailure"
	case SmEventPduSessN1N2TransferFailureIndication:
		return "SmEventPduSessN1N2TransferFailureIndication"
	default:
		return "invalid SM event"
	}
}

func (s SmEventData) String() string {
	return "" // s.Txn.String()
}
