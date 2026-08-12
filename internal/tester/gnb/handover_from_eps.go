// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func (g *GnodeB) WaitForHandoverRequest(timeout time.Duration) (*ngap.HandoverRequest, error) {
	f, err := g.WaitForMessage(Initiating, ngap.ProcHandoverResourceAllocation, timeout)
	if err != nil {
		return nil, err
	}

	req, err := ngap.ParseHandoverRequest(f.Value)
	if err != nil {
		return nil, fmt.Errorf("could not parse Handover Request: %w", err)
	}

	return req, nil
}

type HandoverAdmissionOpts struct {
	Request     *ngap.HandoverRequest
	RANUENGAPID int64

	TargetToSource []byte
}

func (g *GnodeB) AdmitHandover(opts *HandoverAdmissionOpts) error {
	if opts == nil || opts.Request == nil {
		return fmt.Errorf("AdmitHandover needs a Handover Request")
	}

	amfUENGAPID := int64(opts.Request.AMFUENGAPID)

	g.UpdateNGAPIDs(opts.RANUENGAPID, amfUENGAPID)

	admitted := make([]HandoverAdmittedPDUSession, 0, len(opts.Request.PDUSessionResourceSetupListHOReq))

	for _, item := range opts.Request.PDUSessionResourceSetupListHOReq {
		pduSessionID := int64(item.PDUSessionID)

		info, err := getPDUSessionInfoFromSetupRequestTransfer(g, item.Transfer)
		if err != nil {
			return fmt.Errorf("could not read the Handover Request Transfer of PDU session %d: %w", pduSessionID, err)
		}

		info.PDUSessionID = pduSessionID
		info.DLTeid = g.GenerateTEID()

		g.StorePDUSession(opts.RANUENGAPID, info)

		admitted = append(admitted, HandoverAdmittedPDUSession{
			PDUSessionID: pduSessionID,
			DLTeid:       info.DLTeid,
			DLIP:         g.N3Address,
		})

		logger.GnbLogger.Debug("Admitted a PDU session handed over to this node",
			zap.Int64("AMF UE NGAP ID", amfUENGAPID),
			zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
			zap.Int64("PDU Session ID", pduSessionID),
			zap.Uint32("UL TEID", info.ULTeid),
			zap.String("UPF Address", info.UpfAddress),
		)
	}

	return g.SendHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID:                        amfUENGAPID,
		RANUENGAPID:                        opts.RANUENGAPID,
		PDUSessions:                        admitted,
		TargetToSourceTransparentContainer: opts.TargetToSource,
	})
}

func (g *GnodeB) SendHandoverFailure(amfUENGAPID int64, cause ngap.Cause) error {
	msg := &ngap.HandoverFailure{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(amfUENGAPID)),
		Cause:       &cause,
	}

	pdu, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("couldn't build HandoverFailure: %w", err)
	}

	if err := g.SendToRan(pdu, NGAPProcedureHandoverFailure); err != nil {
		return fmt.Errorf("couldn't send HandoverFailure: %w", err)
	}

	logger.GnbLogger.Debug("Sent Handover Failure", zap.Int64("AMF UE NGAP ID", amfUENGAPID))

	return nil
}
