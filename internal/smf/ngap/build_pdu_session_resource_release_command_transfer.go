// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	libngap "github.com/ellanetworks/core/ngap"
)

func BuildPDUSessionResourceReleaseCommandTransfer() ([]byte, error) {
	transfer := libngap.PDUSessionResourceReleaseCommandTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupNAS, Value: libngap.CauseNASNormalRelease},
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not encode pdu session resource release command transfer: %s", err)
	}

	return buf, nil
}
