// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap")

func Dispatch(ctx context.Context, amfInstance *amf.AMF, conn *sctp.SCTPConn, msg []byte) {
	remoteAddress := conn.RemoteAddr()
	localAddress := conn.LocalAddr()

	ran, ok := amfInstance.FindRadioByConn(conn)
	if !ok {
		var err error

		ran, err = amfInstance.NewRadio(conn)
		if err != nil {
			logger.AmfLog.Error("Failed to add a new radio", zap.Error(err))
			return
		}

		logger.AmfLog.Info("Added a new radio", zap.String("address", amf.AddrString(remoteAddress)))
	}

	if len(msg) == 0 {
		logger.From(ctx, ran.Log).Info("RAN close the connection.")
		amfInstance.RemoveRadio(ctx, ran)

		return
	}

	ran.TouchLastSeen()

	ctx, span := tracer.Start(ctx, "ngap/receive",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	pdu, err := ngap.Decoder(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode NGAP message")
		logger.From(ctx, ran.Log).Error("NGAP decode error", zap.Error(err))

		return
	}

	messageType := getMessageType(pdu)

	span.SetAttributes(
		attribute.String("ngap.message_type", string(messageType)),
		attribute.Int("ngap.pdu_category", pdu.Present),
		attribute.Int("ngap.message_size", len(msg)),
		attribute.String("network.protocol.name", "ngap"),
		attribute.String("network.transport", "sctp"),
		attribute.String("network.peer.address", amf.AddrString(remoteAddress)),
		attribute.String("network.local.address", amf.AddrString(localAddress)),
	)

	// Pre-decode NGSetupRequest so the peer's RANNodeName can be applied to
	// ran.Name *before* the inbound event is logged, preserving chronological
	// ordering with the outbound NGSetupResponse.
	var (
		ngSetupDecoded decode.NGSetupRequest
		ngSetupReport  *decode.Report
		haveNGSetup    bool
	)

	if pdu.Present == ngapType.NGAPPDUPresentInitiatingMessage &&
		pdu.InitiatingMessage != nil &&
		pdu.InitiatingMessage.ProcedureCode.Value == ngapType.ProcedureCodeNGSetup {
		ngSetupDecoded, ngSetupReport = decode.DecodeNGSetupRequest(pdu.InitiatingMessage.Value.NGSetupRequest)
		haveNGSetup = true

		if ngSetupDecoded.RANNodeName != "" {
			amfInstance.UpdateRadioName(ran, ngSetupDecoded.RANNodeName)
		}
	}

	amfInstance.LogNetworkEvent(ctx, ran.Conn, messageType, logger.DirectionInbound, msg)

	if haveNGSetup {
		if !handleDecodeReport(ctx, ran, ngSetupReport) {
			return
		}

		HandleNGSetupRequest(ctx, amfInstance, ran, ngSetupDecoded)

		return
	}

	// TS 38.413: NG Setup must be the first NGAP procedure after
	// the TNL association is established. Reject anything else.
	if ran.RanID == nil {
		logger.From(ctx, ran.Log).Error("Received NGAP message before NG Setup, dropping", zap.String("messageType", string(messageType)))

		return
	}

	dispatchNgapMsg(ctx, amfInstance, ran, pdu)
}

func dispatchNgapMsg(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, pdu *ngapType.NGAPPDU) {
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			logger.From(ctx, ran.Log).Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeInitialUEMessage:
			decoded, report := decode.DecodeInitialUEMessage(pdu.InitiatingMessage.Value.InitialUEMessage)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialUEMessage(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkNASTransport:
			decoded, report := decode.DecodeUplinkNASTransport(pdu.InitiatingMessage.Value.UplinkNASTransport)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUplinkNasTransport(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeNGReset:
			decoded, report := decode.DecodeNGReset(pdu.InitiatingMessage.Value.NGReset)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleNGReset(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverCancel:
			decoded, report := decode.DecodeHandoverCancel(pdu.InitiatingMessage.Value.HandoverCancel)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverCancel(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			decoded, report := decode.DecodeUEContextReleaseRequest(pdu.InitiatingMessage.Value.UEContextReleaseRequest)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUEContextReleaseRequest(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			decoded, report := decode.DecodeNASNonDeliveryIndication(pdu.InitiatingMessage.Value.NASNonDeliveryIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleNasNonDeliveryIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeErrorIndication:
			decoded, report := decode.DecodeErrorIndication(pdu.InitiatingMessage.Value.ErrorIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleErrorIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			decoded, report := decode.DecodeUERadioCapabilityInfoIndication(pdu.InitiatingMessage.Value.UERadioCapabilityInfoIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUERadioCapabilityInfoIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverNotification:
			decoded, report := decode.DecodeHandoverNotify(pdu.InitiatingMessage.Value.HandoverNotify)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverNotify(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverPreparation:
			decoded, report := decode.DecodeHandoverRequired(pdu.InitiatingMessage.Value.HandoverRequired)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverRequired(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkRANStatusTransfer:
			decoded, report := decode.DecodeUplinkRANStatusTransfer(pdu.InitiatingMessage.Value.UplinkRANStatusTransfer)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUplinkRanStatusTransfer(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			decoded, report := decode.DecodeRANConfigurationUpdate(pdu.InitiatingMessage.Value.RANConfigurationUpdate)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleRanConfigurationUpdate(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			decoded, report := decode.DecodePDUSessionResourceNotify(pdu.InitiatingMessage.Value.PDUSessionResourceNotify)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceNotify(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePathSwitchRequest:
			decoded, report := decode.DecodePathSwitchRequest(pdu.InitiatingMessage.Value.PathSwitchRequest)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePathSwitchRequest(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeLocationReport:
			decoded, report := decode.DecodeLocationReport(pdu.InitiatingMessage.Value.LocationReport)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleLocationReport(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			decoded, report := decode.DecodeUplinkRANConfigurationTransfer(pdu.InitiatingMessage.Value.UplinkRANConfigurationTransfer)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUplinkRanConfigurationTransfer(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			decoded, report := decode.DecodePDUSessionResourceModifyIndication(pdu.InitiatingMessage.Value.PDUSessionResourceModifyIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceModifyIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
			nrppaTransport := pdu.InitiatingMessage.Value.UplinkUEAssociatedNRPPaTransport
			if nrppaTransport != nil {
				var (
					amfUeNgapID, ranUeNgapID *int64
					nrppaPdu                 []byte
				)

				for _, ie := range nrppaTransport.ProtocolIEs.List {
					switch ie.Id.Value {
					case ngapType.ProtocolIEIDAMFUENGAPID:
						if ie.Value.AMFUENGAPID != nil {
							amfUeNgapID = &ie.Value.AMFUENGAPID.Value
						}
					case ngapType.ProtocolIEIDRANUENGAPID:
						if ie.Value.RANUENGAPID != nil {
							ranUeNgapID = &ie.Value.RANUENGAPID.Value
						}
					case ngapType.ProtocolIEIDNRPPaPDU:
						if ie.Value.NRPPaPDU != nil {
							nrppaPdu = ie.Value.NRPPaPDU.Value
						}
					}
				}

				if nrppaPdu == nil {
					ran.Log.Warn("Uplink NRPPa transport received but NRPPaPDU IE is missing")
					break
				}

				ran.Log.Debug("Uplink NRPPa transport received",
					zap.Int("payload_len", len(nrppaPdu)),
				)

				if amfUeNgapID != nil {
					ran.Log.Debug("Looking up UE by AMF UE NGAP ID",
						zap.Int64("amfUeNgapID", *amfUeNgapID),
					)

					ranUe := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*amfUeNgapID))
					if ranUe == nil {
						ran.Log.Warn("Unknown AMF UE NGAP ID in NRPPa transport",
							zap.Int64("amfUeNgapID", *amfUeNgapID))

						break
					}

					ran.Log.Debug("Found UE by AMF UE NGAP ID",
						zap.Int64("amfUeNgapID", *amfUeNgapID),
						zap.Int64("ranUeNgapID", int64(ranUe.RanUeNgapID)),
					)

					if ranUeNgapID != nil && ranUe.RanUeNgapID != models.RanUeNgapID(*ranUeNgapID) {
						ran.Log.Warn("Inconsistent RAN UE NGAP ID in NRPPa transport",
							zap.Int64("stored", int64(ranUe.RanUeNgapID)),
							zap.Int64("received", *ranUeNgapID))

						break
					}

					if ue := ranUe.UeContext(); ue != nil {
						ue.SetNRPPaMessage(nrppaPdu)
						ran.Log.Debug("Stored NRPPa message in UE context",
							zap.Int64("amfUeNgapID", *amfUeNgapID),
							zap.Int("payloadLen", len(nrppaPdu)),
						)
					} else {
						ran.Log.Warn("No AMF UE context found for NRPPa transport",
							zap.Int64("amfUeNgapID", *amfUeNgapID),
						)
					}
				} else {
					ran.Log.Warn("NRPPa transport received but AMF UE NGAP ID is missing",
						zap.Int("payloadLen", len(nrppaPdu)),
					)
				}
			}
		default:
			logger.From(ctx, ran.Log).Warn("ignoring unsupported procedure", zap.String("kind", "initiating"), zap.Int64("procedureCode", initiatingMessage.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		successfulOutcome := pdu.SuccessfulOutcome
		if successfulOutcome == nil {
			logger.From(ctx, ran.Log).Error("successful Outcome is nil")
			return
		}

		switch successfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeUEContextRelease:
			decoded, report := decode.DecodeUEContextReleaseComplete(pdu.SuccessfulOutcome.Value.UEContextReleaseComplete)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUEContextReleaseComplete(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			decoded, report := decode.DecodePDUSessionResourceReleaseResponse(pdu.SuccessfulOutcome.Value.PDUSessionResourceReleaseResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceReleaseResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeInitialContextSetup:
			decoded, report := decode.DecodeInitialContextSetupResponse(pdu.SuccessfulOutcome.Value.InitialContextSetupResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialContextSetupResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			decoded, report := decode.DecodePDUSessionResourceSetupResponse(pdu.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceSetupResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			decoded, report := decode.DecodePDUSessionResourceModifyResponse(pdu.SuccessfulOutcome.Value.PDUSessionResourceModifyResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceModifyResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			decoded, report := decode.DecodeHandoverRequestAcknowledge(pdu.SuccessfulOutcome.Value.HandoverRequestAcknowledge)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverRequestAcknowledge(ctx, amfInstance, ran, decoded)
		default:
			logger.From(ctx, ran.Log).Warn("ignoring unsupported procedure", zap.String("kind", "successful-outcome"), zap.Int64("procedureCode", successfulOutcome.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		unsuccessfulOutcome := pdu.UnsuccessfulOutcome
		if unsuccessfulOutcome == nil {
			logger.From(ctx, ran.Log).Error("unsuccessful Outcome is nil")
			return
		}

		switch unsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeInitialContextSetup:
			decoded, report := decode.DecodeInitialContextSetupFailure(pdu.UnsuccessfulOutcome.Value.InitialContextSetupFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialContextSetupFailure(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			decoded, report := decode.DecodeHandoverFailure(pdu.UnsuccessfulOutcome.Value.HandoverFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverFailure(ctx, amfInstance, ran, decoded)
		default:
			logger.From(ctx, ran.Log).Warn("ignoring unsupported procedure", zap.String("kind", "unsuccessful-outcome"), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}
