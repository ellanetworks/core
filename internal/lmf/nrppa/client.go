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
// RAN (cell-ID, timing advance, RSRP, RSRQ).
var ecidMeasurementQuantities = []nrppa.MeasurementQuantityValue{
	nrppa.MeasCellID,
	nrppa.MeasTimingAdvanceType1,
	nrppa.MeasRSRP,
	nrppa.MeasRSRQ,
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
	amfUe, ok := c.amf.FindAMFUEBySupi(supi)
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

// MeasurementRejectedError reports that the RAN answered the E-CID measurement
// request with an E-CIDMeasurementInitiationFailure rather than a response. It
// carries the NRPPa cause so callers can log why E-CID was declined.
type MeasurementRejectedError struct {
	Cause nrppa.Cause
}

func (e *MeasurementRejectedError) Error() string {
	return fmt.Sprintf("RAN rejected E-CID measurement: %s", e.Cause)
}

// WaitForMeasurements blocks until the RAN answers the E-CID request for the UE
// (matching measurementID, received at or after notBefore): a response yields
// the mapped radio measurements (cached in the UE context), a failure yields a
// MeasurementRejectedError carrying the cause. It returns a timeout error if
// neither arrives before ctx is cancelled.
func (c *Client) WaitForMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64, notBefore time.Time) (*amf.RadioMeasurements, error) {
	ue, ok := c.amf.FindAMFUEBySupi(supi)
	if !ok {
		return nil, fmt.Errorf("UE not found: %s", supi)
	}

	ticker := time.NewTicker(measurementPollInterval)
	defer ticker.Stop()

	for {
		m, cause := scanMeasurementOutcome(ue.GetNRPPaMessages(), measurementID, notBefore)
		if m != nil {
			ue.SetRadioMeasurements(m)
			return m, nil
		}

		if cause != nil {
			return nil, &MeasurementRejectedError{Cause: *cause}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for NRPPa measurements (measID=%d): %w", measurementID, ctx.Err())
		case <-ticker.C:
		}
	}
}

// measurementPollInterval is how often WaitForMeasurements polls the UE
// context for a matching NRPPa response.
const measurementPollInterval = 50 * time.Millisecond

// scanMeasurementOutcome scans NRPPa messages (newest first) for the RAN's
// answer to the E-CID request identified by measurementID and received at or
// after notBefore. It returns the mapped measurements for a response, or the
// cause for a failure; both nil means no matching message yet.
func scanMeasurementOutcome(messages []amf.NRPPaMessage, measurementID int64, notBefore time.Time) (*amf.RadioMeasurements, *nrppa.Cause) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Timestamp.Before(notBefore) {
			continue
		}

		parsed, err := nrppa.ParsePDU(msg.Payload)
		if err != nil {
			logger.LmfLog.Debug("skipping undecodable NRPPa message", zap.Error(err))
			continue
		}

		switch parsed.Kind {
		case nrppa.KindECIDMeasurementInitiationResponse:
			if parsed.Response != nil && parsed.Response.LMFUEMeasurementID == measurementID {
				return mapECIDResult(parsed.Response.Result), nil
			}
		case nrppa.KindECIDMeasurementInitiationFailure:
			if parsed.Failure != nil && parsed.Failure.LMFUEMeasurementID == measurementID {
				cause := parsed.Failure.Cause
				return nil, &cause
			}
		}
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
