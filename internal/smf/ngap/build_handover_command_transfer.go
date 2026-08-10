// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	libngap "github.com/ellanetworks/core/ngap"
)

// Every IE of the transfer is optional (TS 38.413 §9.3.4.10); the core offers no
// forwarding endpoint, so the source NG-RAN node is given none.
func BuildHandoverCommandTransfer() ([]byte, error) {
	transfer := libngap.HandoverCommandTransfer{}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not encode handover command transfer: %s", err)
	}

	return buf, nil
}
