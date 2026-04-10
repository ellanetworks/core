// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/upf")

// EstablishSession creates a new UPF session from typed Go structs,
// bypassing PFCP message encoding/decoding.
func (conn *SessionEngine) EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/establish_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("models.operation", "establish"),
			attribute.Int64("models.seid", int64(req.LocalSEID)),
			attribute.String("ue.imsi", req.IMSI),
		),
	)
	defer span.End()

	seid := req.LocalSEID
	sess := NewSession(seid)
	span.AddEvent("session_created", trace.WithAttributes(attribute.Int64("models.seid", int64(seid))))

	logger.WithTrace(ctx, logger.UpfLog).Debug("Tracking new session", logger.SEID(seid))

	var createdPDRs []SPDRInfo

	pdrContext := NewPDRCreationContext(sess, conn.FteIDResourceManager)

	farMap := make(map[uint32]ebpf.FarInfo)
	qerMap := make(map[uint32]ebpf.QerInfo)

	bpfObjects := conn.BpfObjects

	for _, far := range req.FARs {
		farInfo := farInfoFromModel(far, conn.n3AddressIPv4, conn.n3AddressIPv6)

		go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

		sess.PutFar(far.FARID, farInfo)
		farMap[far.FARID] = farInfo

		logger.WithTrace(ctx, logger.UpfLog).Info("Created Forwarding Action Rule",
			logger.FARID(far.FARID), zap.Any("farInfo", farInfo))
	}

	for _, qer := range req.QERs {
		qerInfo := qerInfoFromModel(qer)

		sess.NewQer(qer.QERID, qerInfo)
		qerMap[qer.QERID] = qerInfo

		logger.WithTrace(ctx, logger.UpfLog).Info("Created QoS Enforcement Rule",
			logger.QERID(qer.QERID), zap.Any("qerInfo", qerInfo))
	}

	for _, urr := range req.URRs {
		if err := bpfObjects.NewUrr(urr.URRID); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("can't put URR: %w", err)
		}

		logger.WithTrace(ctx, logger.UpfLog).Debug("Created Usage Reporting Rule",
			logger.URRID(urr.URRID),
		)
	}

	for _, pdr := range req.PDRs {
		spdrInfo := SPDRInfo{
			PdrID: uint32(pdr.PDRID),
			PdrInfo: ebpf.PdrInfo{
				SEID:  seid,
				PdrID: uint32(pdr.PDRID),
				IMSI:  req.IMSI,
			},
		}

		if err := pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("couldn't extract PDR info: %w", err)
		}

		if req.PolicyID != 0 {
			dir := models.DirectionUplink
			if spdrInfo.UEIP.IsValid() {
				dir = models.DirectionDownlink
			}

			spdrInfo.PdrInfo.FilterMapIndex = conn.resolveFilterIndex(req.PolicyID, dir)
		}

		sess.PutPDR(spdrInfo.PdrID, spdrInfo)

		if err := applyPDR(spdrInfo, bpfObjects); err != nil {
			if spdrInfo.Allocated {
				pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
			}

			span.RecordError(err)

			return nil, fmt.Errorf("couldn't apply PDR: %w", err)
		}

		logger.WithTrace(ctx, logger.UpfLog).Info("Applied packet detection rule",
			logger.PDRID(spdrInfo.PdrID))

		createdPDRs = append(createdPDRs, spdrInfo)
		bpfObjects.ClearNotified(seid, pdr.PDRID, spdrInfo.PdrInfo.Qer.Qfi)
	}

	span.AddEvent("pdrs_processed", trace.WithAttributes(attribute.Int("count", len(createdPDRs))))
	span.AddEvent("ebpf_maps_updated")

	if req.PolicyID != 0 {
		sess.SetPolicyID(req.PolicyID)
	}

	conn.mu.Lock()
	conn.sessions[seid] = sess
	conn.registerPolicy(req.PolicyID, seid)
	conn.mu.Unlock()

	logger.WithTrace(ctx, logger.UpfLog).Debug("Accepted Session Establishment Request")

	return &models.EstablishResponse{
		RemoteSEID:  seid,
		CreatedPDRs: createdPDRsToResponse(createdPDRs, conn.advertisedN3AddressIPv4, conn.advertisedN3AddressIPv6),
	}, nil
}

// farInfoFromModel converts a models.FAR to an ebpf.FarInfo.
func farInfoFromModel(far models.FAR, localIPv4 netip.Addr, localIPv6 netip.Addr) ebpf.FarInfo {
	info := ebpf.FarInfo{
		Action: encodeApplyAction(far.ApplyAction),
	}

	if fp := far.ForwardingParameters; fp != nil {
		if ohc := fp.OuterHeaderCreation; ohc != nil {
			info.OuterHeaderCreation = uint8(ohc.Description >> 8)
			info.TeID = ohc.TEID

			if ohc.Description == models.OuterHeaderCreationGtpUUdpIpv6 && ohc.IPv6Address != nil {
				info.LocalIP = ebpf.IPToIn6Addr(localIPv6)

				v6 := ohc.IPv6Address.To16()
				if v6 != nil {
					var v6arr [16]byte
					copy(v6arr[:], v6)
					info.RemoteIP = v6arr
				}
			} else if ohc.IPv4Address != nil {
				info.LocalIP = ebpf.IPToIn6Addr(localIPv4)

				ip4 := ohc.IPv4Address.To4()
				if ip4 != nil {
					var ip4arr [4]byte
					copy(ip4arr[:], ip4)
					info.RemoteIP = ebpf.IPToIn6Addr(netip.AddrFrom4(ip4arr))
				}
			} else {
				// No remote IP set yet (e.g. DL FAR before gNB responds) —
				// default to IPv4 local address.
				info.LocalIP = ebpf.IPToIn6Addr(localIPv4)
			}
		}
	}

	return info
}

// encodeApplyAction packs ApplyAction bools into the uint8 bit layout
// expected by the eBPF data plane.
func encodeApplyAction(a models.ApplyAction) uint8 {
	var v uint8
	if a.Drop {
		v |= 0x01
	}

	if a.Forw {
		v |= 0x02
	}

	if a.Buff {
		v |= 0x04
	}

	if a.Nocp {
		v |= 0x08
	}

	if a.Dupl {
		v |= 0x10
	}

	return v
}

// qerInfoFromModel converts a models.QER to an ebpf.QerInfo.
func qerInfoFromModel(qer models.QER) ebpf.QerInfo {
	info := ebpf.QerInfo{
		Qfi: qer.QFI,
	}

	if qer.GateStatus != nil {
		info.GateStatusDL = qer.GateStatus.DLGate
		info.GateStatusUL = qer.GateStatus.ULGate
	}

	if qer.MBR != nil {
		info.MaxBitrateDL = qer.MBR.DLMBR * 1000
		info.MaxBitrateUL = qer.MBR.ULMBR * 1000
	}

	return info
}

// createdPDRsToResponse converts internal SPDRInfo to models.CreatedPDR.
func createdPDRsToResponse(createdPDRs []SPDRInfo, n3IPv4 netip.Addr, n3IPv6 netip.Addr) []models.CreatedPDR {
	var result []models.CreatedPDR

	for _, pdr := range createdPDRs {
		if !pdr.Allocated {
			continue
		}

		// Only uplink PDRs (with allocated TEIDs) are meaningful
		// in the response — the SMF already knows the UE IP.
		if pdr.UEIP.IsValid() {
			continue
		}

		result = append(result, models.CreatedPDR{
			PDRID:  uint16(pdr.PdrID),
			TEID:   pdr.TeID,
			N3IPv4: n3IPv4,
			N3IPv6: n3IPv6,
		})
	}

	return result
}

// addRemoteIPToNeigh adds the given remote IP (as an in6_addr [16]byte) to the kernel
// neighbour table so that GTP encapsulated packets can be forwarded.
func addRemoteIPToNeigh(ctx context.Context, remoteIP [16]byte) {
	var zero [16]byte
	if remoteIP == zero {
		return
	}

	ip := ebpf.In6AddrToIP(remoteIP)
	if !ip.IsValid() {
		return
	}

	if err := kernel.AddNeighbour(ctx, ip); err != nil {
		logger.UpfLog.Warn("could not add gnb IP to neighbour list", logger.IPAddress(ip.String()), zap.Error(err))
	}
}
