// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/cat"
	"github.com/ellanetworks/core/internal/tester/tpdu"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

// UICC is the card that the ME passes a (U)SIM data download to (TS 31.111 clause 7.1.1).
type UICC interface {
	// Envelope runs the ENVELOPE command with data, a BER-TLV, and returns the response data and the status word.
	Envelope(data []byte) (response []byte, sw uint16)
	Fetch() (command []byte, sw uint16)
	TerminalResponse(data []byte) (sw uint16)
}

// The TP-FCS of an SMS-DELIVER-REPORT for RP-ERROR to a (U)SIM data download (TS 23.040 clause 9.2.3.22).
const (
	fcsToolkitBusy       = 0xd4
	fcsDataDownloadError = 0xd5
)

// isDataDownload reports whether an SMS-DELIVER is a (U)SIM data download: TP-PID '7F' and a message of class 2
// (TS 23.040 clause 9.2.3.9, TS 31.111 clause 7.1.1.1).
func isDataDownload(d tpdu.Deliver) bool {
	class, ok := tpdu.MessageClass(d.DCS)
	return d.PID == tpdu.PIDSIMDataDownload && ok && class == 2
}

// envelopeSMSPPDownload is the data of the ENVELOPE (SMS-PP DOWNLOAD) of an SMS-DELIVER, from the service centre at
// serviceCentre, if known (TS 31.111 clause 7.1.1.2).
func envelopeSMSPPDownload(serviceCentre *sms.Address, deliver []byte) ([]byte, error) {
	objects := cat.AppendTLV(nil, cat.TagDeviceIdentities|cat.ComprehensionRequired, []byte{cat.DeviceNetwork, cat.DeviceUICC})

	if serviceCentre != nil {
		// The RP-Originating-Address value, TON and NPI then the BCD digits (TS 24.011 clause 8.2.5.1), is coded as
		// the address object's value is: TON and NPI as for the EF of abbreviated dialling numbers, then the dialling
		// number string (TS 102 223 clause 8.1).
		rpoa, err := serviceCentre.AppendBinary(nil)
		if err != nil {
			return nil, fmt.Errorf("smsue: service centre address: %w", err)
		}

		objects = cat.AppendTLV(objects, cat.TagAddress, rpoa)
	}

	objects = cat.AppendTLV(objects, cat.TagSMSTPDU|cat.ComprehensionRequired, deliver)

	return cat.AppendTLV(nil, cat.TagSMSPPDownload, objects), nil
}

// dataDownload passes a (U)SIM data download to the UICC and returns the RP message that acknowledges it to the
// network, as TS 31.111 clause 7.1.1.1 has the ME do: an RP-ACK, with the UICC's response data, if any, in the
// TP-User-Data and the TP-PID and TP-DCS of the original message, when the UICC answers '90 00', '91 XX', '62 XX', '63 XX'
// or '6F XX'; an RP-ERROR with the TP-FCS "(U)SIM Application Toolkit Busy" for '93 00'; and an RP-ERROR with the TP-FCS
// "(U)SIM data download error" otherwise. The RP-ERROR's RP-Cause is "Protocol error, unspecified", as no other cause
// of TS 24.011 table 8.4 (part 2) applies.
func dataDownload(uicc UICC, rp *sms.RPData, d tpdu.Deliver) (sms.RPMessage, bool, error) {
	envelope, err := envelopeSMSPPDownload(rp.Originator, rp.UserData)
	if err != nil {
		return deliverError(rp, fcsDataDownloadError), false, err
	}

	response, sw := uicc.Envelope(envelope)

	switch sw1 := byte(sw >> 8); {
	case sw == cat.SWOK, sw1 == cat.SWProactive, sw1 == 0x62, sw1 == 0x63, sw1 == 0x6f:
		pid, dcs := d.PID, d.DCS

		report := tpdu.DeliverReportAck{PID: &pid, DCS: &dcs}
		if len(response) > 0 {
			report.UD = response
		}

		userData, err := report.AppendBinary(nil)
		if err != nil {
			return deliverError(rp, fcsDataDownloadError), false, fmt.Errorf("smsue: SMS-DELIVER-REPORT of the UICC's response: %w", err)
		}

		return &sms.RPAck{Direction: nas.DirectionUplink, Reference: rp.Reference, UserData: userData}, cat.ProactivePending(sw), nil
	case sw == cat.SWToolkitBusy:
		return deliverError(rp, fcsToolkitBusy), false, nil
	default:
		return deliverError(rp, fcsDataDownloadError), false, nil
	}
}

// deliverError is the RP-ERROR of an SMS-DELIVER-REPORT of TP-FCS fcs, without optional parameters (TS 23.040 clause
// 9.2.2.1a).
func deliverError(rp *sms.RPData, fcs byte) *sms.RPError {
	return &sms.RPError{
		Direction: nas.DirectionUplink, Reference: rp.Reference, Cause: sms.RPCauseProtocolErrorUnspecified,
		UserData: []byte{0x00, fcs, 0x00},
	}
}
