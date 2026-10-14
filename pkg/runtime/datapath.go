// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"archive/tar"
	"context"
	"errors"
	"net/netip"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/upf"
	"github.com/ellanetworks/core/internal/upf/bpfdump"
	upfengine "github.com/ellanetworks/core/internal/upf/engine"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Datapath interface {
	smf.UPFClient
	ApplyDatapathSettings(settings upf.DatapathSettings) error
	UpdateAdvertisedN3Addresses(v4, v6 netip.Addr)
	DatapathAttachMode() string
	DumpBPF(ctx context.Context, tw *tar.Writer) error
	Close(ctx context.Context)
}

type DatapathParams struct {
	Config           config.Config
	Metrics          prometheus.Registerer
	SMF              upfengine.SMFReportHandler
	DB               *db.Database
	N3IPv4           netip.Addr
	N3IPv6           netip.Addr
	AdvertisedN3IPv4 netip.Addr
	AdvertisedN3IPv6 netip.Addr
	NAT              bool
	FlowAccounting   bool
	LocalSwitch      bool
}

type DatapathFactory func(ctx context.Context, p DatapathParams) (Datapath, error)

func loadConfig(ctx context.Context, rc RuntimeConfig) (config.Config, error) {
	if rc.Config != nil {
		return *rc.Config, nil
	}

	var cfg config.Config

	err := netutil.Retry(ctx, netutil.BindTimeout, netutil.BindInterval,
		func(e error) bool { return errors.Is(e, config.ErrNoInterfaceIP) },
		func() error {
			var vErr error

			cfg, vErr = config.Validate(rc.ConfigPath)

			return vErr
		})

	return cfg, err
}

type upfDatapath struct {
	*smfUPFAdapter
}

func (d *upfDatapath) ApplyDatapathSettings(settings upf.DatapathSettings) error {
	return d.upf.ApplyDatapathSettings(settings)
}

func (d *upfDatapath) UpdateAdvertisedN3Addresses(v4, v6 netip.Addr) {
	d.upf.UpdateAdvertisedN3Addresses(v4, v6)
}

func (d *upfDatapath) DatapathAttachMode() string {
	return d.upf.DatapathAttachMode()
}

func (d *upfDatapath) DumpBPF(ctx context.Context, tw *tar.Writer) error {
	if d.engine == nil || d.engine.BpfObjects == nil {
		return nil
	}

	opts := bpfdump.DumpOptions{
		Exclude:          []string{"nat_ct", "flow_stats", "nocp_map"},
		MaxEntriesPerMap: 10000,
	}

	if _, err := bpfdump.DumpAll(ctx, d.engine.BpfObjects, opts, tw); err != nil {
		logger.EllaLog.Error("supportbundle: bpf dump failed", zap.Error(err))
		return err
	}

	logger.EllaLog.Info("supportbundle: bpf dump completed")

	return nil
}

func (d *upfDatapath) Close(ctx context.Context) {
	d.upf.Close(ctx)
}

func startUPFDatapath(ctx context.Context, p DatapathParams) (Datapath, error) {
	cfg := p.Config

	upfInstance, err := upf.Start(ctx, p.SMF, cfg.Interfaces.N3, p.N3IPv4, p.N3IPv6, p.AdvertisedN3IPv4, p.AdvertisedN3IPv6, cfg.Interfaces.N6, cfg.Datapath.AttachMode, p.NAT, p.FlowAccounting, p.LocalSwitch)
	if err != nil {
		return nil, err
	}

	eng := upfInstance.Engine()

	if eng != nil && p.DB != nil {
		eng.SetBPFObjects(ctx, eng.BpfObjects, p.DB)
	}

	upf.RegisterMetrics(p.Metrics)

	return &upfDatapath{smfUPFAdapter: &smfUPFAdapter{engine: eng, upf: upfInstance}}, nil
}

type engineDatapath struct {
	*smfUPFAdapter
}

func NewEngineDatapath(eng *upfengine.SessionEngine) Datapath {
	return &engineDatapath{smfUPFAdapter: &smfUPFAdapter{engine: eng}}
}

func (d *engineDatapath) ApplyDatapathSettings(upf.DatapathSettings) error {
	return nil
}

func (d *engineDatapath) UpdateAdvertisedN3Addresses(v4, v6 netip.Addr) {
	d.engine.SetAdvertisedN3Addresses(v4, v6)
}

func (d *engineDatapath) DatapathAttachMode() string {
	return "none"
}

func (d *engineDatapath) DumpBPF(context.Context, *tar.Writer) error {
	return nil
}

func (d *engineDatapath) Close(context.Context) {
	if err := d.engine.Close(); err != nil {
		logger.UpfLog.Warn("close session engine", zap.Error(err))
	}
}
