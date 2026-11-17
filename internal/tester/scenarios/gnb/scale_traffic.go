// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"go.uber.org/zap"
)

const (
	scalePingGrace     = 3 * time.Second
	scaleSilentUEsLogs = 5
)

type scalePinger struct {
	ue       *scaleUE
	id       uint16
	src, upf netip.Addr
	sent     atomic.Uint32
	received atomic.Uint32
}

type rttRecorder struct {
	mu   sync.Mutex
	rtts []time.Duration
}

func (r *rttRecorder) add(d time.Duration) {
	r.mu.Lock()
	r.rtts = append(r.rtts, d)
	r.mu.Unlock()
}

func pingJitter(interval time.Duration) time.Duration {
	return interval/2 + rand.N(interval) //nolint:gosec
}

func runScalePingTraffic(ctx context.Context, ues []*scaleUE, p *scaleParams) error {
	dst, err := netip.ParseAddr(p.PingAddress)
	if err != nil {
		return fmt.Errorf("--ping-address: %w", err)
	}

	pingers := make([]*scalePinger, len(ues))
	byGNB := make(map[*gnb.GnodeB]map[uint32]*scalePinger)

	for i, s := range ues {
		src, err := netip.ParseAddr(s.session.UEIPv4)
		if err != nil {
			return fmt.Errorf("UE %s has no IPv4 address: %w", s.ue.UeSecurity.Supi, err)
		}

		upf, err := netip.ParseAddr(s.session.UpfAddress)
		if err != nil {
			return fmt.Errorf("UE %s has no UPF address: %w", s.ue.UeSecurity.Supi, err)
		}

		pingers[i] = &scalePinger{ue: s, id: uint16(rand.N(1 << 16)), src: src, upf: upf} //nolint:gosec

		if byGNB[s.gnb] == nil {
			byGNB[s.gnb] = make(map[uint32]*scalePinger)
		}

		byGNB[s.gnb][s.session.DLTEID] = pingers[i]
		s.gnb.WatchTEID(s.session.DLTEID)
	}

	start := time.Now()
	rtts := &rttRecorder{}

	for g, byTEID := range byGNB {
		g.SetDownlinkHook(func(teid uint32, packet []byte) {
			pg := byTEID[teid]
			if pg == nil {
				return
			}

			_, _, payload, ok := scenarios.ParseICMPEchoReply(packet)
			if !ok || len(payload) < 8 {
				return
			}

			pg.received.Add(1)
			rtts.add(time.Since(start) - time.Duration(binary.BigEndian.Uint64(payload))) //nolint:gosec
		})
	}

	defer func() {
		for g := range byGNB {
			g.SetDownlinkHook(nil)
		}
	}()

	logger.Logger.Info("sending user plane traffic", zap.Int("ues", len(ues)), zap.Duration("interval", p.PingInterval), zap.Duration("duration", p.Hold))

	deadline := start.Add(p.Hold)

	var (
		wg         sync.WaitGroup
		sendErrors atomic.Int64
	)

	for _, pg := range pingers {
		wg.Add(1)

		var fire func()

		fire = func() {
			if ctx.Err() != nil || time.Now().After(deadline) {
				wg.Done()

				return
			}

			seq := uint16(pg.sent.Add(1))                                            //nolint:gosec
			payload := binary.BigEndian.AppendUint64(nil, uint64(time.Since(start))) //nolint:gosec
			packet := scenarios.ICMPEchoRequest(pg.src, dst, pg.id, seq, payload)

			if err := pg.ue.gnb.SendGPDUWithQFI(pg.ue.session.ULTEID, pg.upf, pg.ue.session.QFI, packet); err != nil {
				sendErrors.Add(1)
			}

			time.AfterFunc(pingJitter(p.PingInterval), fire)
		}

		time.AfterFunc(rand.N(p.PingInterval), fire) //nolint:gosec
	}

	wg.Wait()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(scalePingGrace):
	}

	return summarizeScalePingTraffic(pingers, rtts, sendErrors.Load())
}

func summarizeScalePingTraffic(pingers []*scalePinger, rtts *rttRecorder, sendErrors int64) error {
	var (
		sent, received        int64
		lossy, excess, dupIPs int
		silent                []*scalePinger
	)

	ips := make(map[netip.Addr]int, len(pingers))

	for _, pg := range pingers {
		s, r := pg.sent.Load(), pg.received.Load()
		sent += int64(s)
		received += int64(r)

		switch {
		case s > 0 && r == 0:
			silent = append(silent, pg)
		case r < s:
			lossy++
		case r > s:
			excess++
		}

		ips[pg.src]++
		if ips[pg.src] == 2 {
			dupIPs++
		}
	}

	loss := 0.0
	if sent > 0 {
		loss = 100 * float64(sent-received) / float64(sent)
	}

	fields := []zap.Field{
		zap.Int("ues", len(pingers)), zap.Int64("sent", sent), zap.Int64("received", received),
		zap.Float64("loss_percent", loss), zap.Int("silent_ues", len(silent)), zap.Int("lossy_ues", lossy),
		zap.Int("excess_ues", excess), zap.Int("duplicate_ue_ips", dupIPs), zap.Int64("send_errors", sendErrors),
	}

	rtts.mu.Lock()
	if len(rtts.rtts) > 0 {
		fields = append(fields, scenarios.LatencyFields(rtts.rtts)...)
	}
	rtts.mu.Unlock()

	logger.Logger.Info("user plane traffic complete", fields...)

	for _, pg := range silent[:min(len(silent), scaleSilentUEsLogs)] {
		logger.Logger.Warn("UE got no echo reply",
			zap.String("supi", pg.ue.ue.UeSecurity.Supi), zap.Stringer("ue_ip", pg.src), zap.Stringer("upf", pg.upf),
			zap.Uint32("ul_teid", pg.ue.session.ULTEID), zap.Uint32("dl_teid", pg.ue.session.DLTEID),
			zap.Int64("ran_ue_id", pg.ue.ranID), zap.String("gnb", pg.ue.gnb.Name), zap.Uint32("sent", pg.sent.Load()))
	}

	if len(silent) > 0 {
		return fmt.Errorf("%d UEs got no echo reply", len(silent))
	}

	if sendErrors > 0 {
		return fmt.Errorf("%d echo requests could not be sent", sendErrors)
	}

	return nil
}
