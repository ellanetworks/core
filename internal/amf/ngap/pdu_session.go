package ngap

func validPDUSessionID(id int64) (uint8, bool) {
	if id < 1 || id > 15 {
		return 0, false
	}

	return uint8(id), true
}
