// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handlePDUSessionResourceReleaseCommand(gnb *GnodeB, cmd *ngapType.PDUSessionResourceReleaseCommand) error {
	var (
		amfueNGAPID *ngapType.AMFUENGAPID
		ranueNGAPID *ngapType.RANUENGAPID
		nasPDU      *ngapType.NASPDU
		releaseList *ngapType.PDUSessionResourceToReleaseListRelCmd
	)

	for _, ie := range cmd.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDNASPDU:
			nasPDU = ie.Value.NASPDU
		case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
			releaseList = ie.Value.PDUSessionResourceToReleaseListRelCmd
		}
	}

	if amfueNGAPID == nil {
		return fmt.Errorf("missing AMF UE NGAP ID in PDUSessionResourceReleaseCommand")
	}

	if ranueNGAPID == nil {
		return fmt.Errorf("missing RAN UE NGAP ID in PDUSessionResourceReleaseCommand")
	}

	if releaseList == nil {
		return fmt.Errorf("missing PDU Session Resource To Release List in PDUSessionResourceReleaseCommand")
	}

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Release Command",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
		zap.Int("PDU Sessions to release", len(releaseList.List)),
	)

	if nasPDU != nil {
		ue, err := gnb.LoadUE(ranueNGAPID.Value)
		if err != nil {
			return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %v", ranueNGAPID.Value, err)
		}

		if err := ue.SendDownlinkNAS(nasPDU.Value, amfueNGAPID.Value, ranueNGAPID.Value); err != nil {
			return fmt.Errorf("forward NAS PDU for release command: %v", err)
		}
	}

	for _, item := range releaseList.List {
		pduSessionID := item.PDUSessionID.Value
		gnb.RemovePDUSession(ranueNGAPID.Value, pduSessionID)

		logger.GnbLogger.Debug(
			"Released PDU session",
			zap.Int64("PDU Session ID", pduSessionID),
			zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		)
	}

	if err := gnb.SendPDUSessionResourceReleaseResponse(&PDUSessionResourceReleaseResponseOpts{
		AMFUENGAPID:   amfueNGAPID.Value,
		RANUENGAPID:   ranueNGAPID.Value,
		PDUSessionIDs: extractPDUSessionIDs(releaseList),
	}); err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceReleaseResponse: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDU Session Resource Release Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
	)

	return nil
}

func extractPDUSessionIDs(list *ngapType.PDUSessionResourceToReleaseListRelCmd) []int64 {
	ids := make([]int64, 0, len(list.List))
	for _, item := range list.List {
		ids = append(ids, item.PDUSessionID.Value)
	}

	return ids
}
