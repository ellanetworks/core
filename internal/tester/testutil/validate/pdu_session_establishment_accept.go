// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package validate

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/nas/fgs"
)

type ExpectedPDUSessionEstablishmentAccept struct {
	PDUSessionID               fgs.PDUSessionID
	PDUSessionType             fgs.PDUSessionType
	UeIPSubnet                 netip.Prefix
	Dnn                        string
	Sst                        int32
	Sd                         string
	MaximumBitRateUplinkMbps   uint64
	MaximumBitRateDownlinkMbps uint64
	Qfi                        uint8
	FiveQI                     uint8
}

func PDUSessionEstablishmentAccept(plain []byte, opts *ExpectedPDUSessionEstablishmentAccept) error {
	if len(plain) < 4 {
		return fmt.Errorf("NAS message is too short")
	}

	if plain[0] != 46 {
		return fmt.Errorf("extended protocol discriminator not expected value")
	}

	if fgs.GSMMessageType(plain[3]) != fgs.MsgPDUSessionEstablishmentAccept {
		return fmt.Errorf("PDU Session Establishment Accept message type is not correct, expected: %d, got: %d", uint8(fgs.MsgPDUSessionEstablishmentAccept), plain[3])
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(plain)
	if err != nil {
		return fmt.Errorf("could not parse PDU Session Establishment Accept: %v", err)
	}

	if acc.PTI != 1 {
		return fmt.Errorf("pti not expected value")
	}

	if acc.PDUSessionType != opts.PDUSessionType {
		return fmt.Errorf("pdu session type not expected value")
	}

	if len(acc.QoSRules) == 0 {
		return fmt.Errorf("authorized qos rules is missing")
	}

	ambr := acc.SessionAMBR

	if uint64(ambr.Uplink) != opts.MaximumBitRateUplinkMbps {
		return fmt.Errorf("uplink ambr value not expected, got: %d, expected: %d", ambr.Uplink, opts.MaximumBitRateUplinkMbps)
	}

	if uint64(ambr.Downlink) != opts.MaximumBitRateDownlinkMbps {
		return fmt.Errorf("downlink ambr value not expected, got: %d, expected: %d", ambr.Downlink, opts.MaximumBitRateDownlinkMbps)
	}

	if ambr.DownlinkUnit != fgs.SessionAMBRUnit1Mbps {
		return fmt.Errorf("downlink ambr unit not expected, got: %d, expected: %d", ambr.DownlinkUnit, fgs.SessionAMBRUnit1Mbps)
	}

	if ambr.UplinkUnit != fgs.SessionAMBRUnit1Mbps {
		return fmt.Errorf("uplink ambr unit not expected, got: %d, expected: %d", ambr.UplinkUnit, fgs.SessionAMBRUnit1Mbps)
	}

	if acc.PDUSessionID != opts.PDUSessionID {
		return fmt.Errorf("unexpected PDUSessionID: %d", acc.PDUSessionID)
	}

	if acc.PDUAddress == nil {
		return fmt.Errorf("PDU Session Establishment Accept carries no PDU address")
	}

	ueIP := acc.PDUAddress.IPv4Addr()

	if !ueIP.IsValid() {
		// IPv6-only PDU session — no IPv4 address to validate
	} else if !opts.UeIPSubnet.Contains(ueIP) {
		return fmt.Errorf("UE IP %s is not contained in expected subnet %s", ueIP.String(), opts.UeIPSubnet.String())
	}

	qosRules := acc.QoSRules
	if len(qosRules) != 1 {
		return fmt.Errorf("unexpected number of QoS Rules: %d", len(qosRules))
	}

	if qosRules[0].QFI != opts.Qfi {
		return fmt.Errorf("unexpected QoS Rules Identifier: %d, expected: %d", qosRules[0].QFI, opts.Qfi)
	}

	qosFlowDescs := acc.QoSFlowDescriptions
	if len(qosFlowDescs) != 1 {
		return fmt.Errorf("unexpected number of AuthorizedQosFlowDescriptions: %d", len(qosFlowDescs))
	}

	qosFlowDesc := qosFlowDescs[0]

	if qosFlowDesc.QFI != opts.Qfi {
		return fmt.Errorf("unexpected AuthorizedQosFlowDescriptions QFI: %d", qosFlowDesc.QFI)
	}

	if len(qosFlowDesc.Parameters) != 1 {
		return fmt.Errorf("unexpected number of AuthorizedQosFlowDescriptions Parameters: %d, expected: 1", len(qosFlowDesc.Parameters))
	}

	param := qosFlowDesc.Parameters[0]
	if param.ID != fgs.QoSFlowParam5QI {
		return fmt.Errorf("unexpected AuthorizedQosFlowDescriptions Parameter Type: %d, expected: %d", param.ID, fgs.QoSFlowParam5QI)
	}

	if len(param.Value) != 1 || param.Value[0] != opts.FiveQI {
		return fmt.Errorf("unexpected AuthorizedQosFlowDescriptions FiveQI: % x, expected: %d", param.Value, opts.FiveQI)
	}

	dnn := ""

	if acc.DNN != nil {
		dnn = string(*acc.DNN)
	}

	if dnn != opts.Dnn {
		return fmt.Errorf("unexpected DNN: %s", dnn)
	}

	if acc.SNSSAI == nil {
		return fmt.Errorf("S-NSSAI is missing")
	}

	snssai := *acc.SNSSAI
	if snssai.SST != uint8(opts.Sst) {
		return fmt.Errorf("unexpected SNSSAI SST: %d", snssai.SST)
	}

	var sd [3]uint8
	if snssai.SD != nil {
		sd = *snssai.SD
	}

	sdStr := testutil.SDFromNAS(sd)
	if sdStr != opts.Sd {
		return fmt.Errorf("unexpected SNSSAI SD: %s", sdStr)
	}

	return nil
}
