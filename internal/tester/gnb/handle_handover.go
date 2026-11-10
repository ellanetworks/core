// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// handleHandoverRequest checks that the request the AMF sends the target node
// decodes (TS 38.413 §8.4.2). It stores nothing: the target has not assigned a
// RAN UE NGAP ID yet, and the session store is keyed by that ID. AdmitHandover
// takes the request back from WaitForHandoverRequest and stores what it admits
// under the RAN UE NGAP ID the caller chose.
func handleHandoverRequest(gnb *GnodeB, value []byte) error {
	req, err := ngap.ParseHandoverRequest(value)
	if err != nil {
		return fmt.Errorf("undecodable HandoverRequest: %w", err)
	}

	if req.TraceActivation != nil {
		gnb.recordHandoverTraceID(int64(req.AMFUENGAPID), req.TraceActivation.NGRANTraceID)
	}

	return nil
}
