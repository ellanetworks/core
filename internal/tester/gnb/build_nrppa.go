// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/nrppa"
	"github.com/free5gc/ngap/ngapType"
)

// Sample serving-cell parameters used in the gNB tester's E-CID response. These
// mirror the mock coordinates used elsewhere in the tester.
const (
	sampleNRCellIdentity uint64 = 0x000000010 // 36-bit NR cell identity
	sampleTimingAdvance  int64  = 100         // valueTimingAdvanceType1-EUTRA (0..7690)

	// NR-specific sample measurements (TS 38.455 §9.2.5 extension IEs).
	sampleNRTimingAdvance int64 = 100  // Value Timing Advance NR (0..7690)
	sampleUERxTxTimeDiff  int64 = 1200 // UE Rx-Tx Time Difference (0..61565)
	sampleAoAAzimuth      int64 = 900  // 0.1° units → 90.0°
	sampleAoAZenith       int64 = 450  // 0.1° units → 45.0°
)

var (
	// samplePLMNIdentity is PLMN 001/01 in TBCD form (00 f1 10).
	samplePLMNIdentity = []byte{0x00, 0xf1, 0x10}
	// sampleServingCellTAC is a 3-octet Tracking Area Code.
	sampleServingCellTAC = []byte{0x00, 0x00, 0x01}
)

// NRPPaECIDResponseOpts contains the parameters for building the gNB's NRPPa
// E-CIDMeasurementInitiationResponse.
type NRPPaECIDResponseOpts struct {
	AMFUeNgapID        int64
	RANUeNgapID        int64
	LMFUEMeasurementID int64
	RANUEMeasurementID int64
	TimingAdvance      int64 // valueTimingAdvanceType1-EUTRA
}

// BuildNRPPaECIDMeasurementResponse creates an NGAP UplinkUEAssociatedNRPPaTransport
// carrying an NRPPa E-CIDMeasurementInitiationResponse with a sample serving
// NR-CGI, TAC, NG-RANAccessPointPosition and timing-advance measured result.
func BuildNRPPaECIDMeasurementResponse(opts *NRPPaECIDResponseOpts) (ngapType.NGAPPDU, error) {
	if opts == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("NRPPaECIDResponseOpts is nil")
	}

	if opts.AMFUeNgapID == 0 {
		return ngapType.NGAPPDU{}, fmt.Errorf("AMF UE NGAP ID is required")
	}

	if opts.RANUeNgapID == 0 {
		return ngapType.NGAPPDU{}, fmt.Errorf("RAN UE NGAP ID is required")
	}

	nrCell := sampleNRCellIdentity
	ta := opts.TimingAdvance
	nrTA := sampleNRTimingAdvance
	rxTx := sampleUERxTxTimeDiff
	zenith := sampleAoAZenith

	result := &nrppa.ECIDResult{
		ServingCell: nrppa.ServingCell{
			PLMNIdentity:   samplePLMNIdentity,
			NRCellIdentity: &nrCell,
		},
		ServingCellTAC:     sampleServingCellTAC,
		APPosition:         sampleAccessPointPosition(),
		TimingAdvanceType1: &ta,
		NRTimingAdvance:    &nrTA,
		UERxTxTimeDiff:     &rxTx,
		AoA: &nrppa.AoAResult{
			AzimuthRaw: sampleAoAAzimuth,
			ZenithRaw:  &zenith,
		},
	}

	nrppaPdu, err := nrppa.BuildECIDMeasurementInitiationResponse(
		opts.LMFUEMeasurementID,
		opts.RANUEMeasurementID,
		result,
	)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("failed to build NRPPa E-CID response: %w", err)
	}

	return buildUplinkUEAssociatedNRPPaTransport(opts.AMFUeNgapID, opts.RANUeNgapID, nrppaPdu), nil
}

// sampleAccessPointPosition returns a mock NG-RANAccessPointPosition near
// 45.0°N, ~21.5°E (matching the tester's mock geographic coordinates).
func sampleAccessPointPosition() *nrppa.APPosition {
	return &nrppa.APPosition{
		LatitudeSign:           0,       // north
		Latitude:               4194304, // 2^22 → 45.0° via N*90/2^23
		Longitude:              1000000, // ~21.45° via N*360/2^24
		DirectionOfAltitude:    0,       // height
		Altitude:               100,
		UncertaintySemiMajor:   5,
		UncertaintySemiMinor:   5,
		OrientationOfMajorAxis: 0,
		UncertaintyAltitude:    3,
		Confidence:             67,
	}
}

// buildUplinkUEAssociatedNRPPaTransport wraps an NRPPa PDU octet string in an
// NGAP UplinkUEAssociatedNRPPaTransport initiating message.
func buildUplinkUEAssociatedNRPPaTransport(amfUeNgapID, ranUeNgapID int64, nrppaPdu []byte) ngapType.NGAPPDU {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentUplinkUEAssociatedNRPPaTransport
	initiatingMessage.Value.UplinkUEAssociatedNRPPaTransport = new(ngapType.UplinkUEAssociatedNRPPaTransport)

	uplinkNRPPaTransport := initiatingMessage.Value.UplinkUEAssociatedNRPPaTransport
	uplinkNRPPaTransportIEs := &uplinkNRPPaTransport.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.UplinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkUEAssociatedNRPPaTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	uplinkNRPPaTransportIEs.List = append(uplinkNRPPaTransportIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.UplinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkUEAssociatedNRPPaTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	uplinkNRPPaTransportIEs.List = append(uplinkNRPPaTransportIEs.List, ie)

	// NRPPa PDU
	ie = ngapType.UplinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNRPPaPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkUEAssociatedNRPPaTransportIEsPresentNRPPaPDU
	ie.Value.NRPPaPDU = new(ngapType.NRPPaPDU)
	ie.Value.NRPPaPDU.Value = nrppaPdu
	uplinkNRPPaTransportIEs.List = append(uplinkNRPPaTransportIEs.List, ie)

	return pdu
}
