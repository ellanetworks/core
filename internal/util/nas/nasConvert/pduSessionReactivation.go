// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasConvert

func PDUSessionReactivationResultErrorCauseToBuf(errPduSessionId, errCause []uint8) (buf []uint8) {
	if errPduSessionId == nil || len(errPduSessionId) != len(errCause) {
		return
	}
	for i := 0; i < len(errPduSessionId); i++ {
		buf = append(buf, errPduSessionId[i])
		buf = append(buf, errCause[i])
	}
	return
}
