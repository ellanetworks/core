// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nrppa provides the LMF's NRPPa client for communicating with the RAN.
// The client sends NRPPa PDUs via NGAP transport through the AMF, then reads
// responses from the AMF's UE context.
package nrppa

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nrppa"
	"go.uber.org/zap"
)

// Client communicates with the RAN via NRPPa procedures through the AMF.
type Client struct {
	amf     *amf.AMF
	measSeq atomic.Int64
}

// New creates a new NRPPa client backed by the given AMF.
func New(amfInstance *amf.AMF) *Client {
	return &Client{amf: amfInstance}
}

// nextMeasurementID returns the next LMF-UE-Measurement-ID, cycled within the
// 1..15 root range of UE-Measurement-ID (TS 38.455).
func (c *Client) nextMeasurementID() int64 {
	return (c.measSeq.Add(1)-1)%15 + 1
}

// ecidMeasurementQuantities are the E-CID quantities the LMF requests from the
// RAN. Cell-ID comes from AMF location context; timing advance and AP position
// are optional NRPPa enhancements.
//
// For an NR cell the relevant quantities are the NR-specific ones (TS 38.455):
// SS/CSI RSRP/RSRQ for signal level, and angleOfArrivalNR / timingAdvanceNR /
// uE-Rx-Tx-Time-Diff for angle and timing. The legacy rSRP/rSRQ are kept for
// gNBs that only report the generic quantities. We deliberately do NOT request
// the E-UTRA timingAdvanceType1/type2 — an NR gNB reports timingAdvanceNR
// instead. The gNB returns whichever quantities it supports and omits the rest.
var ecidMeasurementQuantities = []nrppa.MeasurementQuantityValue{
	nrppa.MeasRSRP,
	nrppa.MeasRSRQ,
	nrppa.MeasSSRSRP,
	nrppa.MeasSSRSRQ,
	nrppa.MeasCSIRSRP,
	nrppa.MeasCSIRSRQ,
	nrppa.MeasAngleOfArrivalNR,
	nrppa.MeasTimingAdvanceNR,
	nrppa.MeasUERxTxTimeDiff,
}

// RequestMeasurements sends an NRPPa E-CIDMeasurementInitiationRequest to the
// RAN for the given UE. The PDU is encoded and sent via NGAP downlink
// UE-associated NRPPa transport.
//
// It returns the LMF-UE-Measurement-ID assigned to the request so the caller
// can match the asynchronous E-CIDMeasurementInitiationResponse via
// WaitForMeasurements. The method parameter is accepted for API compatibility;
// only E-CID is supported in this MVP.
func (c *Client) RequestMeasurements(ctx context.Context, supi etsi.SUPI, method string) (int64, error) {
	amfUe, ok := c.amf.FindUeContextBySupi(supi)
	if !ok {
		return 0, fmt.Errorf("UE not found: %s", supi)
	}

	ranUe := amfUe.RanUe()
	if ranUe == nil {
		return 0, fmt.Errorf("UE has no active RAN connection: %s", supi)
	}

	ran := ranUe.Radio()
	if ran == nil || ran.NGAPSender == nil {
		return 0, fmt.Errorf("UE has no NGAP sender available: %s", supi)
	}

	measID := c.nextMeasurementID()

	payload, err := nrppa.BuildECIDMeasurementInitiationRequest(measID, ecidMeasurementQuantities)
	if err != nil {
		return 0, fmt.Errorf("failed to build NRPPa E-CID request: %w", err)
	}

	err = ran.NGAPSender.SendDownlinkNRPPaTransport(
		ctx,
		ranUe.AmfUeNgapID,
		ranUe.RanUeNgapID,
		0, // RoutingID: 0 for MVP (not used by gNB tester)
		payload,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to send NRPPa transport: %w", err)
	}

	logger.LmfLog.Debug("NRPPa E-CID measurement request sent",
		zap.String("supi", supi.String()),
		zap.String("method", method),
		zap.Int64("lmfMeasurementID", measID),
	)

	return measID, nil
}

// WaitForMeasurements blocks until an NRPPa E-CIDMeasurementInitiationResponse
// or E-CIDMeasurementInitiationFailure matching measurementID arrives for the
// UE (received at or after notBefore), or ctx is cancelled/times out.
//
// On success it maps the E-CID measurement result to radio measurements, caches
// them in the UE context, and returns them. On RAN rejection it returns nil
// immediately so the caller can fall back to Cell ID without waiting for a
// timeout.
func (c *Client) WaitForMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64, notBefore time.Time) (*amf.RadioMeasurements, error) {
	ue, ok := c.amf.FindUeContextBySupi(supi)
	if !ok {
		return nil, fmt.Errorf("UE not found: %s", supi)
	}

	ticker := time.NewTicker(measurementPollInterval)
	defer ticker.Stop()

	for {
		resp, fail := matchMeasurementResponse(ue.GetNRPPaMessages(), measurementID, notBefore)
		if resp != nil {
			// Some gNBs (observed with a real O-CU-CP) do not echo the
			// LMF-assigned LMF-UE-Measurement-ID back in the response; they
			// keep returning the id of the first/active measurement instead.
			// Because the LMF issues exactly one on-demand E-CID request per UE
			// at a time and only considers responses received at/after
			// notBefore, the newest such response is unambiguously ours — so we
			// accept it and just log the id discrepancy rather than time out.
			if resp.LMFUEMeasurementID != measurementID {
				logger.LmfLog.Warn("gNB returned a different LMF-UE-Measurement-ID; accepting newest fresh E-CID response",
					zap.String("supi", supi.String()),
					zap.Int64("expectedMeasurementID", measurementID),
					zap.Int64("receivedMeasurementID", resp.LMFUEMeasurementID),
				)
			}

			m := mapECIDResult(resp.Result)
			ue.SetRadioMeasurements(m)

			// Release the measurement association in the gNB so subsequent
			// on-demand requests are treated as fresh measurements. Best-effort:
			// the E-CID fix is already complete, so failures here are logged but
			// not propagated. Use the ids the gNB actually reported.
			c.terminateMeasurement(ctx, supi, resp.LMFUEMeasurementID, resp.RANUEMeasurementID)

			return m, nil
		}

		if fail != nil {
			logger.LmfLog.Warn("E-CID measurement rejected by RAN; falling back to Cell ID",
				zap.String("supi", supi.String()),
				zap.Int64("lmfMeasurementID", measurementID),
				zap.Int("causeGroup", int(fail.Cause.Group)),
				zap.Int64("causeValue", fail.Cause.Value),
			)

			return nil, fmt.Errorf("E-CID measurement rejected by RAN (cause=%d/%d)", fail.Cause.Group, fail.Cause.Value)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for NRPPa measurements (measID=%d): %w", measurementID, ctx.Err())
		case <-ticker.C:
		}
	}
}

// terminateMeasurement sends an NRPPa E-CIDMeasurementTerminationCommand for the
// given measurement to the RAN, releasing the measurement association in the
// gNB. The Termination Command is a Class 2 procedure (no response), so this is
// fire-and-forget. Errors are logged and swallowed: by the time it is called the
// E-CID fix has already been obtained.
func (c *Client) terminateMeasurement(ctx context.Context, supi etsi.SUPI, lmfMeasID, ranMeasID int64) {
	amfUe, ok := c.amf.FindUeContextBySupi(supi)
	if !ok {
		return
	}

	ranUe := amfUe.RanUe()
	if ranUe == nil {
		return
	}

	ran := ranUe.Radio()
	if ran == nil || ran.NGAPSender == nil {
		return
	}

	payload, err := nrppa.BuildECIDMeasurementTerminationCommand(lmfMeasID, ranMeasID)
	if err != nil {
		logger.LmfLog.Warn("failed to build NRPPa E-CID termination command",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return
	}

	if err := ran.NGAPSender.SendDownlinkNRPPaTransport(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, 0, payload); err != nil {
		logger.LmfLog.Warn("failed to send NRPPa E-CID termination command",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return
	}

	logger.LmfLog.Debug("NRPPa E-CID measurement terminated",
		zap.String("supi", supi.String()),
		zap.Int64("lmfMeasurementID", lmfMeasID),
		zap.Int64("ranMeasurementID", ranMeasID),
	)
}

// measurementPollInterval is how often WaitForMeasurements polls the UE
// context for a matching NRPPa response.
const measurementPollInterval = 50 * time.Millisecond

// matchMeasurementResponse scans NRPPa messages (newest first) for an
// E-CIDMeasurementInitiationResponse or E-CIDMeasurementInitiationFailure
// received at or after notBefore.
//
// Correlation prefers an exact LMF-UE-Measurement-ID match, but falls back to
// the newest fresh E-CID Initiation message when no exact match exists. The
// fallback is necessary because some gNBs do not echo the LMF-assigned
// measurement id; since the LMF has exactly one outstanding on-demand E-CID
// request per UE and only considers messages at/after notBefore, the newest
// such message is unambiguously the response to the current request.
//
// Returns:
//   - (response, nil) on success
//   - (nil, failure) when the RAN explicitly rejected the request
//   - (nil, nil) when no fresh response or failure was found
func matchMeasurementResponse(messages []amf.NRPPaMessage, measurementID int64, notBefore time.Time) (*nrppa.ECIDResponse, *nrppa.ECIDFailure) {
	var (
		fallbackResponse *nrppa.ECIDResponse
		fallbackFailure  *nrppa.ECIDFailure
	)

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Timestamp.Before(notBefore) {
			continue
		}

		parsed, err := nrppa.ParsePDU(msg.Payload)
		if err != nil {
			logger.LmfLog.Debug("NRPPa ParsePDU failed, retrying with first byte stripped",
				zap.Error(err),
				zap.Int("payloadLen", len(msg.Payload)),
			)
			// Fallback: try parsing with first byte stripped.
			// Some gNBs include an extra length byte or the AMF may add
			// a PER OCTET STRING length determinant byte.
			if len(msg.Payload) > 1 {
				parsed, err = nrppa.ParsePDU(msg.Payload[1:])
				if err != nil {
					logger.LmfLog.Debug("NRPPa ParsePDU also failed with first byte stripped",
						zap.Error(err),
						zap.Int("payloadLen", len(msg.Payload)-1),
					)

					continue
				}
			} else {
				continue
			}
		}

		if parsed.Response != nil && parsed.Kind == nrppa.KindECIDMeasurementInitiationResponse {
			if parsed.Response.LMFUEMeasurementID == measurementID {
				return parsed.Response, nil
			}
			// Newest fresh response (scan is newest-first) becomes the fallback.
			if fallbackResponse == nil && fallbackFailure == nil {
				fallbackResponse = parsed.Response
			}

			continue
		}

		// Both an Initiation Failure (the RAN rejects the request outright) and a
		// Failure Indication (the RAN accepted it but can no longer provide the
		// measurement, TS 38.455 §9.1.3) terminate the wait with the RAN's cause.
		if parsed.Failure != nil && (parsed.Kind == nrppa.KindECIDMeasurementInitiationFailure ||
			parsed.Kind == nrppa.KindECIDMeasurementFailureIndication) {
			if parsed.Failure.LMFUEMeasurementID == measurementID {
				return nil, parsed.Failure
			}

			if fallbackResponse == nil && fallbackFailure == nil {
				fallbackFailure = parsed.Failure
			}

			continue
		}
	}

	// No exact LMF-UE-Measurement-ID match: fall back to the newest fresh
	// E-CID Initiation message received for this UE.
	if fallbackResponse != nil {
		return fallbackResponse, nil
	}

	if fallbackFailure != nil {
		return nil, fallbackFailure
	}

	return nil, nil
}

// mapECIDResult converts a decoded E-CID measurement result into the AMF radio
// measurement shape. Timing advance is taken from valueTimingAdvanceType1 (or
// type2 as fallback); RSRP/RSRQ are left nil unless reported. The serving cell
// access point position is carried through when present.
func mapECIDResult(result *nrppa.ECIDResult) *amf.RadioMeasurements {
	m := &amf.RadioMeasurements{}

	if result == nil {
		return m
	}

	switch {
	case result.TimingAdvanceType1 != nil:
		ta := int32(*result.TimingAdvanceType1)
		m.TA = &ta
	case result.TimingAdvanceType2 != nil:
		ta := int32(*result.TimingAdvanceType2)
		m.TA = &ta
	}

	// NR timing measurements (TS 38.455 §9.2.5 extension IEs).
	if result.NRTimingAdvance != nil {
		nrta := int32(*result.NRTimingAdvance)
		m.NRTimingAdvance = &nrta
	}

	if result.UERxTxTimeDiff != nil {
		rxtx := int32(*result.UERxTxTimeDiff)
		m.RxTxTimeDifference = &rxtx
	}

	// NR Angle of Arrival (azimuth, optional zenith), decimal degrees.
	if result.AoA != nil {
		az := result.AoA.AzimuthDegrees
		m.AoAAzimuthDegrees = &az

		if result.AoA.ZenithDegrees != nil {
			ze := *result.AoA.ZenithDegrees
			m.AoAZenithDegrees = &ze
		}
	}

	// Map NR-specific measurements (SSB/CSI-RS based)
	if result.ResultSSRSRP != nil {
		if len(result.ResultSSRSRP.Items) > 0 {
			// Use the first (strongest) SS-RSRP measurement
			ssrsrp := ssrsrpToDBm(result.ResultSSRSRP.Items[0].Value)
			m.SSRSRP = &ssrsrp
		}
	}

	if result.ResultSSRSRQ != nil {
		if len(result.ResultSSRSRQ.Items) > 0 {
			ssrsrq := ssrsrqToDB(result.ResultSSRSRQ.Items[0].Value)
			m.SSRSRQ = &ssrsrq
		}
	}

	if result.ResultCSIRSRP != nil {
		if len(result.ResultCSIRSRP.Items) > 0 {
			csirsrp := csirsrpToDBm(result.ResultCSIRSRP.Items[0].Value)
			m.CSIRSRP = &csirsrp
		}
	}

	if result.ResultCSIRSRQ != nil {
		if len(result.ResultCSIRSRQ.Items) > 0 {
			csirsrq := csirsrqToDB(result.ResultCSIRSRQ.Items[0].Value)
			m.CSIRSRQ = &csirsrq
		}
	}

	if result.APPosition != nil {
		m.APPosition = &amf.APPosition{
			LatitudeDegrees:      result.APPosition.LatitudeDegrees,
			LongitudeDegrees:     result.APPosition.LongitudeDegrees,
			Altitude:             result.APPosition.Altitude,
			UncertaintySemiMajor: result.APPosition.UncertaintySemiMajor,
			UncertaintySemiMinor: result.APPosition.UncertaintySemiMinor,
			Confidence:           result.APPosition.Confidence,
		}
	}

	return m
}

// ssrsrpToDBm converts an NR SS-RSRP report value (0..127) to dBm × 100.
// Per TS 38.133 Table 10.1.6.1-1 (NR, not the E-UTRA table):
//
//	SS-RSRP_00  : SS-RSRP < -156 dBm
//	SS-RSRP_k   : (-157 + k) ≤ SS-RSRP < (-156 + k) dBm   (k = 1..126)
//	SS-RSRP_127 : SS-RSRP ≥ -30 dBm
//
// We report the lower bound of the band (dBm × 100).
func ssrsrpToDBm(v int64) int32 {
	if v <= 0 {
		return -15600 // < -156 dBm
	}

	return int32(v-157) * 100
}

// ssrsrqToDB converts an NR SS-RSRQ report value (0..127) to dB × 100.
// Per TS 38.133 Table 10.1.11.1-1 (NR, range -43..+20 dB, 0.5 dB step):
//
//	SS-RSRQ_00  : SS-RSRQ < -43 dB
//	SS-RSRQ_k   : (-43 + (k-1)*0.5) ≤ SS-RSRQ < (-43 + k*0.5) dB   (k = 1..126)
//	SS-RSRQ_127 : SS-RSRQ ≥ 20 dB
//
// We report the lower bound of the band (dB × 100).
func ssrsrqToDB(v int64) int32 {
	if v <= 0 {
		return -4300 // < -43 dB
	}

	return int32(-4300 + (v-1)*50)
}

// csirsrpToDBm converts an NR CSI-RSRP report value to dBm × 100. CSI-RSRP
// shares the SS-RSRP report mapping (TS 38.133 Table 10.1.6.1-1).
// NOTE: not validated against a live capture.
func csirsrpToDBm(v int64) int32 {
	if v <= 0 {
		return -15600 // < -156 dBm
	}

	return int32(v-157) * 100
}

// csirsrqToDB converts an NR CSI-RSRQ report value to dB × 100. CSI-RSRQ shares
// the SS-RSRQ report mapping (TS 38.133 Table 10.1.11.1-1).
func csirsrqToDB(v int64) int32 {
	return ssrsrqToDB(v)
}
