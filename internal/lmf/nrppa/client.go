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
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	coremodels "github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nrppa"
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
	measID := c.nextMeasurementID()

	payload, err := nrppa.BuildECIDMeasurementInitiationRequest(measID, ecidMeasurementQuantities)
	if err != nil {
		return 0, fmt.Errorf("failed to build NRPPa E-CID request: %w", err)
	}

	// RoutingID 0 for MVP (single LMF). A CM-IDLE UE is paged and the request buffered.
	if err := c.amf.TransferN2NRPPaMsg(ctx, supi, 0, payload); err != nil {
		return 0, fmt.Errorf("failed to transfer NRPPa E-CID request: %w", err)
	}

	logger.LmfLog.Debug("NRPPa E-CID measurement request sent",
		logger.SUPI(supi.String()),
		zap.String("method", method),
		zap.Int64("lmf_measurement_id", measID),
	)

	return measID, nil
}

// CancelMeasurements discards a request buffered for a paged UE, once the LMF stops
// waiting. Safe to call unconditionally.
func (c *Client) CancelMeasurements(ctx context.Context, supi etsi.SUPI, _ int64) {
	c.amf.CancelBufferedN1N2(ctx, supi, "", coremodels.N2ClassNRPPa)
}

// WaitForMeasurements blocks until an NRPPa E-CIDMeasurementInitiationResponse
// or E-CIDMeasurementInitiationFailure matching measurementID arrives for the
// UE (received at or after notBefore), or ctx is cancelled/times out.
//
// On success it maps the E-CID measurement result to radio measurements, caches
// them in the UE context, and returns them. On RAN rejection it returns nil
// immediately so the caller can fall back to Cell ID without waiting for a
// timeout.
func (c *Client) WaitForMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64, notBefore time.Time) (*lmfmodels.RadioMeasurements, error) {
	ue, ok := c.amf.LookupUeBySupi(supi)
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
					logger.SUPI(supi.String()),
					zap.Int64("expected_measurement_id", measurementID),
					zap.Int64("received_measurement_id", resp.LMFUEMeasurementID),
				)
			}

			// On-demand E-CID: per TS 38.455 §8.2.1.2 the NG-RAN node considers the
			// measurement terminated once it returns the Initiation Response, so no
			// Termination Command is sent (that procedure is for periodic reporting,
			// §8.2.4.1).
			m := mapECIDResult(resp.Result)
			ue.SetRadioMeasurements(m)

			return m, nil
		}

		if fail != nil {
			logger.LmfLog.Warn("E-CID measurement rejected by RAN; falling back to Cell ID",
				logger.SUPI(supi.String()),
				zap.Int64("lmf_measurement_id", measurementID),
				zap.Int("cause_group", int(fail.Cause.Group)),
				zap.Int64("cause_value", fail.Cause.Value),
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
				zap.Int("payload_len", len(msg.Payload)),
			)
			// Fallback: try parsing with first byte stripped.
			// Some gNBs include an extra length byte or the AMF may add
			// a PER OCTET STRING length determinant byte.
			if len(msg.Payload) > 1 {
				parsed, err = nrppa.ParsePDU(msg.Payload[1:])
				if err != nil {
					logger.LmfLog.Debug("NRPPa ParsePDU also failed with first byte stripped",
						zap.Error(err),
						zap.Int("payload_len", len(msg.Payload)-1),
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

// mapECIDResult converts a decoded E-CID measurement result into the shared
// radio-measurement shape. Timing advance is taken from valueTimingAdvanceType1
// (or type2 as fallback); RSRP/RSRQ are left nil unless reported. The serving
// cell access point position is carried through when present.
func mapECIDResult(result *nrppa.ECIDResult) *lmfmodels.RadioMeasurements {
	m := &lmfmodels.RadioMeasurements{}

	if result == nil {
		return m
	}

	cells := lmfmodels.NewCellCollector(lmfmodels.MeasurementSourceNetwork)

	for _, it := range result.RSRP {
		cell := cells.Cell(lmfmodels.RATEUTRA, it.PCI, it.EARFCN)
		cell.ECGI = eutraCGI(it.CGI)
		v := lmfmodels.EUTRARSRPDBm(it.ValueRSRP)
		cell.RSRP = &v
	}

	for _, it := range result.RSRQ {
		cell := cells.Cell(lmfmodels.RATEUTRA, it.PCI, it.EARFCN)
		cell.ECGI = eutraCGI(it.CGI)
		v := lmfmodels.EUTRARSRQDB(it.ValueRSRQ)
		cell.RSRQ = &v
	}

	for _, it := range result.SSRSRP {
		cell := nrCell(cells, it.NRPCI, it.NRARFCN, it.CGI)
		cell.SSRSRP = nrRSRP(it.Value)

		for _, b := range it.PerSSB {
			lmfmodels.Beam(&cell.SSBBeams, b.SSBIndex).RSRP = lmfmodels.NRRSRPDBm(b.Value)
		}
	}

	for _, it := range result.SSRSRQ {
		cell := nrCell(cells, it.NRPCI, it.NRARFCN, it.CGI)
		cell.SSRSRQ = nrValue(it.Value, lmfmodels.NRRSRQDB)

		for _, b := range it.PerSSB {
			v := lmfmodels.NRRSRQDB(b.Value)
			lmfmodels.Beam(&cell.SSBBeams, b.SSBIndex).RSRQ = &v
		}
	}

	for _, it := range result.CSIRSRP {
		cell := nrCell(cells, it.NRPCI, it.NRARFCN, it.CGI)
		cell.CSIRSRP = nrRSRP(it.Value)

		for _, b := range it.PerCSIRS {
			lmfmodels.Beam(&cell.CSIRSBeams, b.CSIRSIndex).RSRP = lmfmodels.NRRSRPDBm(b.Value)
		}
	}

	for _, it := range result.CSIRSRQ {
		cell := nrCell(cells, it.NRPCI, it.NRARFCN, it.CGI)
		cell.CSIRSRQ = nrValue(it.Value, lmfmodels.NRRSRQDB)

		for _, b := range it.PerCSIRS {
			v := lmfmodels.NRRSRQDB(b.Value)
			lmfmodels.Beam(&cell.CSIRSBeams, b.CSIRSIndex).RSRQ = &v
		}
	}

	if serving := servingCell(cells, result.ServingCell); serving != nil {
		switch {
		case result.TimingAdvanceType1 != nil:
			serving.TimingAdvance = result.TimingAdvanceType1
		case result.TimingAdvanceType2 != nil:
			serving.TimingAdvance = result.TimingAdvanceType2
		}

		serving.NRTimingAdvance = result.NRTimingAdvance
		serving.UERxTxTimeDiff = result.UERxTxTimeDiff

		switch {
		case result.AoA != nil:
			az := result.AoA.AzimuthDegrees
			serving.AoAAzimuthDegrees = &az
			serving.AoAZenithDegrees = result.AoA.ZenithDegrees
		case result.AngleOfArrival != nil:
			az := float64(*result.AngleOfArrival) * 0.5
			serving.AoAAzimuthDegrees = &az
		}
	}

	m.Cells = cells.Cells()

	if result.APPosition != nil {
		altitude := result.APPosition.Altitude
		if result.APPosition.DirectionOfAltitude == 1 { // depth (below the ellipsoid)
			altitude = -altitude
		}

		m.APPosition = lmfmodels.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{
			LatitudeDegrees:      result.APPosition.LatitudeDegrees,
			LongitudeDegrees:     result.APPosition.LongitudeDegrees,
			AltitudeMeters:       float64(altitude),
			UncertaintySemiMajor: result.APPosition.UncertaintySemiMajor,
			UncertaintySemiMinor: result.APPosition.UncertaintySemiMinor,
			OrientationMajor:     result.APPosition.OrientationOfMajorAxis,
			UncertaintyAltitude:  result.APPosition.UncertaintyAltitude,
			Confidence:           result.APPosition.Confidence,
		}.Estimate()
	}

	return m
}

func nrCell(cells *lmfmodels.CellCollector, pci, arfcn int64, cgi *nrppa.CGINR) *lmfmodels.CellMeasurement {
	cell := cells.Cell(lmfmodels.RATNR, pci, arfcn)

	if cgi != nil {
		if plmn, err := lmfmodels.PlmnFromOctets(cgi.PLMNIdentity); err == nil {
			cell.NCGI = lmfmodels.NewNcgi(plmn, cgi.NRCellIdentity)
		}
	}

	return cell
}

func eutraCGI(cgi *nrppa.CGIEUTRA) *coremodels.Ecgi {
	if cgi == nil {
		return nil
	}

	plmn, err := lmfmodels.PlmnFromOctets(cgi.PLMNIdentity)
	if err != nil {
		return nil
	}

	return lmfmodels.NewEcgi(plmn, cgi.EUTRACellID)
}

func nrRSRP(v *int64) *float64 {
	if v == nil {
		return nil
	}

	return lmfmodels.NRRSRPDBm(*v)
}

func nrValue(v *int64, convert func(int64) float64) *float64 {
	if v == nil {
		return nil
	}

	out := convert(*v)

	return &out
}

func servingCell(cells *lmfmodels.CellCollector, cgi nrppa.NGRANCGI) *lmfmodels.CellMeasurement {
	plmn, err := lmfmodels.PlmnFromOctets(cgi.PLMNIdentity)
	if err != nil {
		return nil
	}

	switch {
	case cgi.NRCellIdentity != nil:
		return cells.Serving(lmfmodels.RATNR, lmfmodels.NewNcgi(plmn, *cgi.NRCellIdentity), nil)
	case cgi.EUTRACellID != nil:
		return cells.Serving(lmfmodels.RATEUTRA, nil, lmfmodels.NewEcgi(plmn, *cgi.EUTRACellID))
	default:
		return nil
	}
}
