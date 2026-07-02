// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"fmt"

	"github.com/ellanetworks/core/internal/nrppa/nrppatype"
	"github.com/free5gc/aper"
)

// transactionIDFor derives a deterministic NRPPATransactionID (0..32767) from
// the LMF-UE-Measurement-ID so a request and its matching response/failure share
// the same transaction id. The MVP correlates primarily by measurement id.
func transactionIDFor(lmfMeasID int64) nrppatype.NRPPATransactionID {
	return nrppatype.NRPPATransactionID{Value: lmfMeasID & 0x7fff}
}

// BuildECIDMeasurementInitiationRequest builds an E-CIDMeasurementInitiation
// InitiatingMessage PDU (onDemand) requesting the given measurement quantities
// and returns the aligned-PER bytes. lmfMeasID is the LMF-assigned
// UE-Measurement-ID (1..15 root range) used for correlation.
func BuildECIDMeasurementInitiationRequest(lmfMeasID int64, quantities []MeasurementQuantityValue) ([]byte, error) {
	if len(quantities) == 0 {
		return nil, fmt.Errorf("at least one measurement quantity is required")
	}

	req := &nrppatype.ECIDMeasurementInitiationRequest{}
	list := &req.ProtocolIEs.List

	// id-LMF-UE-Measurement-ID (mandatory, reject).
	{
		ie := nrppatype.ECIDMeasurementInitiationRequestIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDLMFUEMeasurementID
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationRequestIEsPresentLMFUEMeasurementID
		ie.Value.LMFUEMeasurementID = &nrppatype.UEMeasurementID{Value: lmfMeasID}
		*list = append(*list, ie)
	}

	// id-ReportCharacteristics (mandatory, reject) — onDemand.
	{
		ie := nrppatype.ECIDMeasurementInitiationRequestIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDReportCharacteristics
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationRequestIEsPresentReportCharacteristics
		ie.Value.ReportCharacteristics = &nrppatype.ReportCharacteristics{
			Value: nrppatype.ReportCharacteristicsPresentOnDemand,
		}
		*list = append(*list, ie)
	}

	// id-MeasurementQuantities (mandatory, reject).
	{
		mq := &nrppatype.MeasurementQuantities{}

		for _, q := range quantities {
			item := nrppatype.MeasurementQuantitiesIEs{}
			item.Id.Value = nrppatype.ProtocolIEIDMeasurementQuantitiesItem
			item.Criticality = reject()
			item.Value.Present = nrppatype.MeasurementQuantitiesIEsPresentMeasurementQuantitiesItem
			item.Value.MeasurementQuantitiesItem = &nrppatype.MeasurementQuantitiesItem{
				MeasurementQuantitiesValue: nrppatype.MeasurementQuantitiesValue{
					Value: aper.Enumerated(q),
				},
			}
			mq.List = append(mq.List, item)
		}

		ie := nrppatype.ECIDMeasurementInitiationRequestIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDMeasurementQuantities
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationRequestIEsPresentMeasurementQuantities
		ie.Value.MeasurementQuantities = mq
		*list = append(*list, ie)
	}

	pdu := nrppatype.NRPPaPDU{
		Present: nrppatype.NRPPaPDUPresentInitiatingMessage,
		InitiatingMessage: &nrppatype.InitiatingMessage{
			ProcedureCode:      nrppatype.ProcedureCode{Value: nrppatype.ProcedureCodeECIDMeasurementInitiation},
			Criticality:        reject(),
			NRPPATransactionID: transactionIDFor(lmfMeasID),
			Value: nrppatype.InitiatingMessageValue{
				Present:                          nrppatype.InitiatingMessagePresentECIDMeasurementInitiationRequest,
				ECIDMeasurementInitiationRequest: req,
			},
		},
	}

	return Encoder(pdu)
}

// BuildECIDMeasurementInitiationResponse builds an E-CIDMeasurementInitiation
// SuccessfulOutcome PDU carrying the given E-CID-MeasurementResult and returns
// the aligned-PER bytes. This is the gNB/RAN side of the procedure.
func BuildECIDMeasurementInitiationResponse(lmfMeasID, ranMeasID int64, result *ECIDResult) ([]byte, error) {
	resp := &nrppatype.ECIDMeasurementInitiationResponse{}
	list := &resp.ProtocolIEs.List

	// id-LMF-UE-Measurement-ID (mandatory, reject).
	{
		ie := nrppatype.ECIDMeasurementInitiationResponseIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDLMFUEMeasurementID
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationResponseIEsPresentLMFUEMeasurementID
		ie.Value.LMFUEMeasurementID = &nrppatype.UEMeasurementID{Value: lmfMeasID}
		*list = append(*list, ie)
	}

	// id-RAN-UE-Measurement-ID (mandatory, reject).
	{
		ie := nrppatype.ECIDMeasurementInitiationResponseIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDRANUEMeasurementID
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationResponseIEsPresentRANUEMeasurementID
		ie.Value.RANUEMeasurementID = &nrppatype.UEMeasurementID{Value: ranMeasID}
		*list = append(*list, ie)
	}

	// id-E-CID-MeasurementResult (optional, ignore).
	if result != nil {
		mr, err := buildECIDMeasurementResult(result)
		if err != nil {
			return nil, fmt.Errorf("build E-CID-MeasurementResult: %w", err)
		}

		ie := nrppatype.ECIDMeasurementInitiationResponseIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDECIDMeasurementResult
		ie.Criticality = ignore()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationResponseIEsPresentECIDMeasurementResult
		ie.Value.ECIDMeasurementResult = mr
		*list = append(*list, ie)
	}

	pdu := nrppatype.NRPPaPDU{
		Present: nrppatype.NRPPaPDUPresentSuccessfulOutcome,
		SuccessfulOutcome: &nrppatype.SuccessfulOutcome{
			ProcedureCode:      nrppatype.ProcedureCode{Value: nrppatype.ProcedureCodeECIDMeasurementInitiation},
			Criticality:        reject(),
			NRPPATransactionID: transactionIDFor(lmfMeasID),
			Value: nrppatype.SuccessfulOutcomeValue{
				Present:                           nrppatype.SuccessfulOutcomePresentECIDMeasurementInitiationResponse,
				ECIDMeasurementInitiationResponse: resp,
			},
		},
	}

	return Encoder(pdu)
}

// BuildECIDMeasurementInitiationFailure builds an E-CIDMeasurementInitiation
// UnsuccessfulOutcome PDU carrying the given Cause.
func BuildECIDMeasurementInitiationFailure(lmfMeasID int64, cause Cause) ([]byte, error) {
	fail := &nrppatype.ECIDMeasurementInitiationFailure{}
	list := &fail.ProtocolIEs.List

	// id-LMF-UE-Measurement-ID (mandatory, reject).
	{
		ie := nrppatype.ECIDMeasurementInitiationFailureIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDLMFUEMeasurementID
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationFailureIEsPresentLMFUEMeasurementID
		ie.Value.LMFUEMeasurementID = &nrppatype.UEMeasurementID{Value: lmfMeasID}
		*list = append(*list, ie)
	}

	// id-Cause (mandatory, ignore).
	{
		ie := nrppatype.ECIDMeasurementInitiationFailureIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDCause
		ie.Criticality = ignore()
		ie.Value.Present = nrppatype.ECIDMeasurementInitiationFailureIEsPresentCause
		ie.Value.Cause = buildCause(cause)
		*list = append(*list, ie)
	}

	pdu := nrppatype.NRPPaPDU{
		Present: nrppatype.NRPPaPDUPresentUnsuccessfulOutcome,
		UnsuccessfulOutcome: &nrppatype.UnsuccessfulOutcome{
			ProcedureCode:      nrppatype.ProcedureCode{Value: nrppatype.ProcedureCodeECIDMeasurementInitiation},
			Criticality:        reject(),
			NRPPATransactionID: transactionIDFor(lmfMeasID),
			Value: nrppatype.UnsuccessfulOutcomeValue{
				Present:                          nrppatype.UnsuccessfulOutcomePresentECIDMeasurementInitiationFailure,
				ECIDMeasurementInitiationFailure: fail,
			},
		},
	}

	return Encoder(pdu)
}

// BuildECIDMeasurementTerminationCommand builds an E-CIDMeasurementTermination
// Command InitiatingMessage PDU (procedureCode = 5). It releases the measurement
// association in the RAN identified by lmfMeasID/ranMeasID. This is a Class 2
// procedure (no response) and is sent LMF → RAN.
func BuildECIDMeasurementTerminationCommand(lmfMeasID, ranMeasID int64) ([]byte, error) {
	cmd := &nrppatype.ECIDMeasurementTerminationCommand{}
	list := &cmd.ProtocolIEs.List

	// id-LMF-UE-Measurement-ID (mandatory, reject).
	{
		ie := nrppatype.ECIDMeasurementTerminationCommandIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDLMFUEMeasurementID
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementTerminationCommandIEsPresentLMFUEMeasurementID
		ie.Value.LMFUEMeasurementID = &nrppatype.UEMeasurementID{Value: lmfMeasID}
		*list = append(*list, ie)
	}

	// id-RAN-UE-Measurement-ID (mandatory, reject).
	{
		ie := nrppatype.ECIDMeasurementTerminationCommandIEs{}
		ie.Id.Value = nrppatype.ProtocolIEIDRANUEMeasurementID
		ie.Criticality = reject()
		ie.Value.Present = nrppatype.ECIDMeasurementTerminationCommandIEsPresentRANUEMeasurementID
		ie.Value.RANUEMeasurementID = &nrppatype.UEMeasurementID{Value: ranMeasID}
		*list = append(*list, ie)
	}

	pdu := nrppatype.NRPPaPDU{
		Present: nrppatype.NRPPaPDUPresentInitiatingMessage,
		InitiatingMessage: &nrppatype.InitiatingMessage{
			ProcedureCode:      nrppatype.ProcedureCode{Value: nrppatype.ProcedureCodeECIDMeasurementTermination},
			Criticality:        reject(),
			NRPPATransactionID: transactionIDFor(lmfMeasID),
			Value: nrppatype.InitiatingMessageValue{
				Present:                           nrppatype.InitiatingMessagePresentECIDMeasurementTerminationCommand,
				ECIDMeasurementTerminationCommand: cmd,
			},
		},
	}

	return Encoder(pdu)
}

// buildCause maps a caller-facing Cause to the aper Cause CHOICE.
func buildCause(c Cause) *nrppatype.Cause {
	out := &nrppatype.Cause{}

	switch c.Group {
	case CauseGroupRadioNetwork:
		out.Present = nrppatype.CausePresentRadioNetwork
		out.RadioNetwork = &nrppatype.CauseRadioNetwork{Value: aper.Enumerated(c.Value)}
	case CauseGroupProtocol:
		out.Present = nrppatype.CausePresentProtocol
		out.Protocol = &nrppatype.CauseProtocol{Value: aper.Enumerated(c.Value)}
	case CauseGroupMisc:
		out.Present = nrppatype.CausePresentMisc
		out.Misc = &nrppatype.CauseMisc{Value: aper.Enumerated(c.Value)}
	default:
		// Fall back to radioNetwork/unspecified for unsupported groups.
		out.Present = nrppatype.CausePresentRadioNetwork
		out.RadioNetwork = &nrppatype.CauseRadioNetwork{Value: nrppatype.CauseRadioNetworkPresentUnspecified}
	}

	return out
}

// buildECIDMeasurementResult maps a caller-facing ECIDResult to the aper type.
func buildECIDMeasurementResult(r *ECIDResult) (*nrppatype.ECIDMeasurementResult, error) {
	if len(r.ServingCell.PLMNIdentity) != 3 {
		return nil, fmt.Errorf("PLMN identity must be 3 octets, got %d", len(r.ServingCell.PLMNIdentity))
	}

	if len(r.ServingCellTAC) != 3 {
		return nil, fmt.Errorf("serving cell TAC must be 3 octets, got %d", len(r.ServingCellTAC))
	}

	cgi := nrppatype.NGRANCGI{
		PLMNIdentity: nrppatype.PLMNIdentity{Value: aper.OctetString(r.ServingCell.PLMNIdentity)},
	}

	switch {
	case r.ServingCell.NRCellIdentity != nil:
		cgi.NGRANCell.Present = nrppatype.NGRANCellPresentNRCellID
		cgi.NGRANCell.NRCellID = &nrppatype.NRCellIdentity{Value: uint64ToBitString(*r.ServingCell.NRCellIdentity, 36)}
	case r.ServingCell.EUTRACellID != nil:
		cgi.NGRANCell.Present = nrppatype.NGRANCellPresentEUTRACellID
		cgi.NGRANCell.EUTRACellID = &nrppatype.EUTRACellIdentifier{Value: uint64ToBitString(*r.ServingCell.EUTRACellID, 28)}
	default:
		return nil, fmt.Errorf("serving cell must carry an NR or E-UTRA cell identity")
	}

	mr := &nrppatype.ECIDMeasurementResult{
		ServingCellID:  cgi,
		ServingCellTAC: nrppatype.TAC{Value: aper.OctetString(r.ServingCellTAC)},
	}

	if r.APPosition != nil {
		mr.NGRANAccessPointPosition = buildAccessPointPosition(r.APPosition)
	}

	measured := &nrppatype.MeasuredResults{}

	if r.TimingAdvanceType1 != nil {
		v := *r.TimingAdvanceType1
		measured.List = append(measured.List, nrppatype.MeasuredResultsValue{
			Present:                      nrppatype.MeasuredResultsValuePresentValueTimingAdvanceType1EUTRA,
			ValueTimingAdvanceType1EUTRA: &v,
		})
	}

	if r.TimingAdvanceType2 != nil {
		v := *r.TimingAdvanceType2
		measured.List = append(measured.List, nrppatype.MeasuredResultsValue{
			Present:                      nrppatype.MeasuredResultsValuePresentValueTimingAdvanceType2EUTRA,
			ValueTimingAdvanceType2EUTRA: &v,
		})
	}

	// Append NR measurement types (SS-RSRP, SS-RSRQ, CSI-RSRP, CSI-RSRQ) as
	// choice-Extension IEs, matching the ASN.1 in TS 38.455 §9.3.5. Each entry
	// is a ProtocolIE-Single-Container with id / criticality / open-type value
	// so that the aper codec dispatches on the ProtocolIEID.

	if r.ResultSSRSRP != nil {
		ssrsrp := &nrppatype.ResultSSRSRP{}

		for _, item := range r.ResultSSRSRP.Items {
			value := item.Value
			ssrsrp.List = append(ssrsrp.List, nrppatype.ResultSSRSRPItem{
				NRPCI:           nrppatype.NRPCI{Value: item.NRPCI},
				NRARFCN:         nrppatype.NRARFCN{Value: 0},
				ValueSSRSRPCell: &value,
			})
		}

		measured.List = append(measured.List, nrppatype.MeasuredResultsValue{
			Present: nrppatype.MeasuredResultsValuePresentChoiceExtension,
			ChoiceExtension: &nrppatype.ProtocolIESingleContainerMeasuredResultsValueExtensionIE{
				MeasuredResultsValueExtIEs: &nrppatype.MeasuredResultsValueExtIEs{
					Id:          nrppatype.ProtocolIEID{Value: nrppatype.ProtocolIEIDResultSSRSRP},
					Criticality: nrppatype.Criticality{Value: nrppatype.CriticalityPresentIgnore},
					Value: nrppatype.MeasuredResultsValueExtIEsValue{
						Present:      nrppatype.MeasuredResultsValueExtIEsPresentResultSSRSRP,
						ResultSSRSRP: ssrsrp,
					},
				},
			},
		})
	}

	if r.ResultSSRSRQ != nil {
		ssrsrq := &nrppatype.ResultSSRSRQ{}

		for _, item := range r.ResultSSRSRQ.Items {
			value := item.Value
			ssrsrq.List = append(ssrsrq.List, nrppatype.ResultSSRSRQItem{
				NRPCI:           nrppatype.NRPCI{Value: item.NRPCI},
				NRARFCN:         nrppatype.NRARFCN{Value: 0},
				ValueSSRSRQCell: &value,
			})
		}

		measured.List = append(measured.List, nrppatype.MeasuredResultsValue{
			Present: nrppatype.MeasuredResultsValuePresentChoiceExtension,
			ChoiceExtension: &nrppatype.ProtocolIESingleContainerMeasuredResultsValueExtensionIE{
				MeasuredResultsValueExtIEs: &nrppatype.MeasuredResultsValueExtIEs{
					Id:          nrppatype.ProtocolIEID{Value: nrppatype.ProtocolIEIDResultSSRSRQ},
					Criticality: nrppatype.Criticality{Value: nrppatype.CriticalityPresentIgnore},
					Value: nrppatype.MeasuredResultsValueExtIEsValue{
						Present:      nrppatype.MeasuredResultsValueExtIEsPresentResultSSRSRQ,
						ResultSSRSRQ: ssrsrq,
					},
				},
			},
		})
	}

	if r.ResultCSIRSRP != nil {
		csirsrp := &nrppatype.ResultCSIRSRP{}

		for _, item := range r.ResultCSIRSRP.Items {
			ie := nrppatype.ResultCSIRSRPItem{
				NRPCI:        nrppatype.NRPCI{Value: item.NRPCI},
				NRARFCN:      nrppatype.NRARFCN{Value: 0},
				ValueCSIRSRP: nrppatype.ValueCSIRSRP{Value: item.Value},
			}
			if item.CSIRSIndex != 0 {
				idx := item.CSIRSIndex
				ie.CSIRSIndex = &idx
			}

			csirsrp.List = append(csirsrp.List, ie)
		}

		measured.List = append(measured.List, nrppatype.MeasuredResultsValue{
			Present: nrppatype.MeasuredResultsValuePresentChoiceExtension,
			ChoiceExtension: &nrppatype.ProtocolIESingleContainerMeasuredResultsValueExtensionIE{
				MeasuredResultsValueExtIEs: &nrppatype.MeasuredResultsValueExtIEs{
					Id:          nrppatype.ProtocolIEID{Value: nrppatype.ProtocolIEIDResultCSIRSRP},
					Criticality: nrppatype.Criticality{Value: nrppatype.CriticalityPresentIgnore},
					Value: nrppatype.MeasuredResultsValueExtIEsValue{
						Present:       nrppatype.MeasuredResultsValueExtIEsPresentResultCSIRSRP,
						ResultCSIRSRP: csirsrp,
					},
				},
			},
		})
	}

	if r.ResultCSIRSRQ != nil {
		csirsrq := &nrppatype.ResultCSIRSRQ{}

		for _, item := range r.ResultCSIRSRQ.Items {
			ie := nrppatype.ResultCSIRSRQItem{
				NRPCI:        nrppatype.NRPCI{Value: item.NRPCI},
				NRARFCN:      nrppatype.NRARFCN{Value: 0},
				ValueCSIRSRQ: nrppatype.ValueCSIRSRQ{Value: item.Value},
			}
			if item.CSIRSIndex != 0 {
				idx := item.CSIRSIndex
				ie.CSIRSIndex = &idx
			}

			csirsrq.List = append(csirsrq.List, ie)
		}

		measured.List = append(measured.List, nrppatype.MeasuredResultsValue{
			Present: nrppatype.MeasuredResultsValuePresentChoiceExtension,
			ChoiceExtension: &nrppatype.ProtocolIESingleContainerMeasuredResultsValueExtensionIE{
				MeasuredResultsValueExtIEs: &nrppatype.MeasuredResultsValueExtIEs{
					Id:          nrppatype.ProtocolIEID{Value: nrppatype.ProtocolIEIDResultCSIRSRQ},
					Criticality: nrppatype.Criticality{Value: nrppatype.CriticalityPresentIgnore},
					Value: nrppatype.MeasuredResultsValueExtIEsValue{
						Present:       nrppatype.MeasuredResultsValueExtIEsPresentResultCSIRSRQ,
						ResultCSIRSRQ: csirsrq,
					},
				},
			},
		})
	}

	if len(measured.List) > 0 {
		mr.MeasuredResults = measured
	}

	return mr, nil
}

// buildAccessPointPosition maps a caller-facing APPosition to the aper type.
func buildAccessPointPosition(p *APPosition) *nrppatype.NGRANAccessPointPosition {
	return &nrppatype.NGRANAccessPointPosition{
		LatitudeSign:           aper.Enumerated(p.LatitudeSign),
		Latitude:               p.Latitude,
		Longitude:              p.Longitude,
		DirectionOfAltitude:    aper.Enumerated(p.DirectionOfAltitude),
		Altitude:               p.Altitude,
		UncertaintySemiMajor:   p.UncertaintySemiMajor,
		UncertaintySemiMinor:   p.UncertaintySemiMinor,
		OrientationOfMajorAxis: p.OrientationOfMajorAxis,
		UncertaintyAltitude:    p.UncertaintyAltitude,
		Confidence:             p.Confidence,
	}
}

// uint64ToBitString packs the low numBits of v into a top-aligned BitString.
func uint64ToBitString(v uint64, numBits int) aper.BitString {
	byteLen := (numBits + 7) / 8
	bytes := make([]byte, byteLen)

	// Left-align the value within numBits (MSB first).
	shifted := v << uint(byteLen*8-numBits)
	for i := byteLen - 1; i >= 0; i-- {
		bytes[i] = byte(shifted & 0xff)
		shifted >>= 8
	}

	return aper.BitString{Bytes: bytes, BitLength: uint64(numBits)}
}
