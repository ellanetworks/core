// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lppa provides the LMF's LPPa client for communicating with the RAN.
// The client sends LPPa PDUs via S1AP transport through the MME, then reads
// responses from the MME's UE context.
//
// LPPa is transported as an octet string inside S1AP UE-associated transport
// messages (TS 36.413 §8.14) between the E-SMLC/LMF and the eNB. This package
// builds the E-CID Measurement Initiation request, sends it, and maps the eNB's
// response to the LMF's radio-measurement shape.
//
// Request/response correlation is by the E-SMLC-UE-Measurement-ID (the
// LMF-assigned measurement id), echoed by the eNB.
package lppa

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/lppa"
	"go.uber.org/zap"
)

// Client communicates with the RAN via LPPa procedures through the MME.
type Client struct {
	mme     *mme.MME
	measSeq atomic.Int64
}

// New creates a new LPPa client backed by the given MME.
func New(mmeInstance *mme.MME) *Client {
	return &Client{mme: mmeInstance}
}

// nextMeasurementID returns the next E-SMLC-UE-Measurement-ID, cycled within the
// 1..15 root range of Measurement-ID (TS 36.455).
func (c *Client) nextMeasurementID() int64 {
	return (c.measSeq.Add(1)-1)%15 + 1
}

// ecidMeasurementQuantities are the E-CID quantities the LMF requests from the
// RAN. Cell-ID comes from the MME location context; angle of arrival, timing
// advance and signal level are optional LPPa enhancements (TS 36.455). The eNB
// returns whichever quantities it supports and omits the rest.
var ecidMeasurementQuantities = []lppa.MeasurementQuantityValue{
	lppa.MeasCellID,
	lppa.MeasAngleOfArrival,
	lppa.MeasTimingAdvanceType1,
	lppa.MeasTimingAdvanceType2,
	lppa.MeasRSRP,
	lppa.MeasRSRQ,
}

// RequestMeasurements sends an LPPa E-CIDMeasurementInitiationRequest to the RAN
// for the given UE. The PDU is encoded and sent via S1AP downlink UE-associated
// LPPa transport.
//
// It returns the E-SMLC-UE-Measurement-ID assigned to the request so the caller
// can match the asynchronous E-CIDMeasurementInitiationResponse via
// WaitForMeasurements.
func (c *Client) RequestMeasurements(ctx context.Context, supi etsi.SUPI, method string) (int64, error) {
	ue, ok := c.mme.LookupUeBySupi(supi)
	if !ok {
		return 0, fmt.Errorf("UE not found: %s", supi)
	}

	conn := ue.Conn()
	if conn == nil {
		return 0, fmt.Errorf("UE has no active S1 connection: %s", supi)
	}

	measID := c.nextMeasurementID()

	payload, err := lppa.BuildECIDMeasurementInitiationRequest(measID, ecidMeasurementQuantities)
	if err != nil {
		return 0, fmt.Errorf("failed to build LPPa E-CID request: %w", err)
	}

	if err := conn.SendDownlinkLPPaTransport(ctx, 0, payload); err != nil {
		return 0, fmt.Errorf("failed to send LPPa transport: %w", err)
	}

	logger.LmfLog.Debug("LPPa E-CID measurement request sent",
		zap.String("supi", supi.String()),
		zap.String("method", method),
		zap.Int64("esmlcMeasurementID", measID),
	)

	return measID, nil
}

// WaitForMeasurements blocks until an LPPa E-CIDMeasurementInitiationResponse or
// a failure matching measurementID arrives for the UE (received at or after
// notBefore), or ctx is cancelled/times out.
//
// On success it maps the E-CID measurement result to radio measurements, caches
// them in the UE context, and returns them. On RAN rejection it returns nil
// immediately so the caller can fall back to Cell ID without waiting for a
// timeout.
func (c *Client) WaitForMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64, notBefore time.Time) (*lmfmodels.RadioMeasurements, error) {
	ue, ok := c.mme.LookupUeBySupi(supi)
	if !ok {
		return nil, fmt.Errorf("UE not found: %s", supi)
	}

	ticker := time.NewTicker(measurementPollInterval)
	defer ticker.Stop()

	for {
		resp, fail := matchMeasurementResponse(ue.GetLPPaMessages(), measurementID, notBefore)
		if resp != nil {
			// Some eNBs do not echo the E-SMLC-assigned Measurement-ID back in the
			// response. Because the LMF issues exactly one on-demand E-CID request
			// per UE at a time and only considers responses received at/after
			// notBefore, the newest such response is unambiguously ours — so we
			// accept it and just log the id discrepancy rather than time out.
			if resp.ESMLCUEMeasurementID != measurementID {
				logger.LmfLog.Warn("eNB returned a different E-SMLC-UE-Measurement-ID; accepting newest fresh E-CID response",
					zap.String("supi", supi.String()),
					zap.Int64("expectedMeasurementID", measurementID),
					zap.Int64("receivedMeasurementID", resp.ESMLCUEMeasurementID),
				)
			}

			// On-demand E-CID: per TS 36.455 §8.2.1.2 the eNB considers the
			// measurement terminated once it returns the Initiation Response, so no
			// Termination Command is sent (that procedure is for periodic reporting,
			// §8.2.4.1).
			m := mapECIDResult(resp.Result)
			ue.SetRadioMeasurements(m)

			return m, nil
		}

		if fail != nil {
			logger.LmfLog.Warn("E-CID measurement rejected by RAN; falling back to Cell ID",
				zap.String("supi", supi.String()),
				zap.Int64("esmlcMeasurementID", measurementID),
				zap.Int("causeGroup", int(fail.Cause.Group)),
				zap.Int64("causeValue", fail.Cause.Value),
			)

			return nil, fmt.Errorf("E-CID measurement rejected by RAN (cause=%d/%d)", fail.Cause.Group, fail.Cause.Value)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for LPPa measurements (measID=%d): %w", measurementID, ctx.Err())
		case <-ticker.C:
		}
	}
}

// measurementPollInterval is how often WaitForMeasurements polls the UE context
// for a matching LPPa response.
const measurementPollInterval = 50 * time.Millisecond

// matchMeasurementResponse scans LPPa messages (newest first) for an
// E-CIDMeasurementInitiationResponse or a failure received at or after notBefore.
//
// Correlation prefers an exact E-SMLC-UE-Measurement-ID match, but falls back to
// the newest fresh E-CID message when no exact match exists, since some eNBs do
// not echo the assigned measurement id and the LMF has exactly one outstanding
// on-demand E-CID request per UE.
//
// Returns:
//   - (response, nil) on success
//   - (nil, failure) when the RAN explicitly rejected the request
//   - (nil, nil) when no fresh response or failure was found
func matchMeasurementResponse(messages []mme.LPPaMessage, measurementID int64, notBefore time.Time) (*lppa.ECIDResponse, *lppa.ECIDFailure) {
	var (
		fallbackResponse *lppa.ECIDResponse
		fallbackFailure  *lppa.ECIDFailure
	)

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Timestamp.Before(notBefore) {
			continue
		}

		parsed, err := lppa.ParsePDU(msg.Payload)
		if err != nil {
			logger.LmfLog.Debug("LPPa ParsePDU failed, retrying with first byte stripped",
				zap.Error(err),
				zap.Int("payloadLen", len(msg.Payload)),
			)
			// Fallback: some eNBs prepend an extra length octet; retry stripped.
			if len(msg.Payload) > 1 {
				parsed, err = lppa.ParsePDU(msg.Payload[1:])
				if err != nil {
					logger.LmfLog.Debug("LPPa ParsePDU also failed with first byte stripped",
						zap.Error(err),
						zap.Int("payloadLen", len(msg.Payload)-1),
					)

					continue
				}
			} else {
				continue
			}
		}

		if parsed.Response != nil && parsed.Kind == lppa.KindECIDMeasurementInitiationResponse {
			if parsed.Response.ESMLCUEMeasurementID == measurementID {
				return parsed.Response, nil
			}

			if fallbackResponse == nil && fallbackFailure == nil {
				fallbackResponse = parsed.Response
			}

			continue
		}

		if fail := failureOf(parsed); fail != nil {
			if fail.ESMLCUEMeasurementID == measurementID {
				return nil, fail
			}

			if fallbackResponse == nil && fallbackFailure == nil {
				fallbackFailure = fail
			}

			continue
		}
	}

	if fallbackResponse != nil {
		return fallbackResponse, nil
	}

	if fallbackFailure != nil {
		return nil, fallbackFailure
	}

	return nil, nil
}

// failureOf returns a common failure view for an Initiation Failure (the RAN
// rejects the request) or a Failure Indication (the RAN accepted it but can no
// longer provide the measurement, TS 36.455 §8.2.3), or nil for other PDUs.
func failureOf(parsed *lppa.ParsedPDU) *lppa.ECIDFailure {
	switch parsed.Kind {
	case lppa.KindECIDMeasurementInitiationFailure:
		return parsed.Failure
	case lppa.KindECIDMeasurementFailureIndication:
		if parsed.FailureIndication == nil {
			return nil
		}

		return &lppa.ECIDFailure{
			ESMLCUEMeasurementID: parsed.FailureIndication.ESMLCUEMeasurementID,
			Cause:                parsed.FailureIndication.Cause,
		}
	default:
		return nil
	}
}

// mapECIDResult converts a decoded LPPa E-CID measurement result into the shared
// radio-measurement shape. Timing advance is taken from valueTimingAdvanceType1
// (or type2 as fallback); RSRP/RSRQ use the strongest reported cell; the serving
// cell access point position is carried through when present.
func mapECIDResult(result *lppa.ECIDResult) *lmfmodels.RadioMeasurements {
	m := &lmfmodels.RadioMeasurements{}

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

	if result.AngleOfArrival != nil {
		// valueAngleOfArrival is 0..719 in 0.5-degree units (TS 36.455 §9.2.x).
		az := float64(*result.AngleOfArrival) * 0.5
		m.AoAAzimuthDegrees = &az
	}

	if len(result.RSRP) > 0 {
		rsrp := valueRSRPToDBm(strongestRSRP(result.RSRP))
		m.RSRP = &rsrp
	}

	if len(result.RSRQ) > 0 {
		rsrq := valueRSRQToDB(strongestRSRQ(result.RSRQ))
		m.RSRQ = &rsrq
	}

	if result.APPosition != nil {
		altitude := result.APPosition.Altitude
		if result.APPosition.DirectionOfAltitude == 1 { // depth (below the ellipsoid)
			altitude = -altitude
		}

		m.APPosition = &lmfmodels.APPosition{
			LatitudeDegrees:      result.APPosition.LatitudeDegrees,
			LongitudeDegrees:     result.APPosition.LongitudeDegrees,
			Altitude:             altitude,
			UncertaintySemiMajor: result.APPosition.UncertaintySemiMajor,
			UncertaintySemiMinor: result.APPosition.UncertaintySemiMinor,
			Confidence:           result.APPosition.Confidence,
		}
	}

	return m
}

// strongestRSRP returns the highest ValueRSRP in the E-CID result list. The
// LPPa ResultRSRP list is not spec-ordered by strength, and the serving cell is
// not tagged, so the strongest cell is used as the best available proxy.
func strongestRSRP(items []lppa.RSRPItem) int64 {
	best := items[0].ValueRSRP

	for _, it := range items[1:] {
		if it.ValueRSRP > best {
			best = it.ValueRSRP
		}
	}

	return best
}

func strongestRSRQ(items []lppa.RSRQItem) int64 {
	best := items[0].ValueRSRQ

	for _, it := range items[1:] {
		if it.ValueRSRQ > best {
			best = it.ValueRSRQ
		}
	}

	return best
}

// valueRSRPToDBm converts an E-UTRA ValueRSRP report (0..97) to dBm × 100.
// Per TS 36.133 Table 9.1.4-1, RSRP_n maps to the band [-141+n, -140+n) dBm; we
// report the band's lower bound.
func valueRSRPToDBm(v int64) int32 {
	return int32((-141 + v) * 100)
}

// valueRSRQToDB converts an E-UTRA ValueRSRQ report (0..34) to dB × 100.
// Per TS 36.133 Table 9.1.7-1, RSRQ_n maps to the band [-20+0.5n, -19.5+0.5n) dB;
// we report the band's lower bound.
func valueRSRQToDB(v int64) int32 {
	return int32(-2000 + 50*v)
}
