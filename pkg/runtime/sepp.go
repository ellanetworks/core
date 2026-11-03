// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func newSEPP(dbInstance *db.Database, n32 config.N32Interface) *sepp.Manager {
	m := sepp.New(sepp.Config{
		Partners:  seppPartnersSource(dbInstance),
		Identity:  seppIdentitySource(dbInstance),
		Operator:  seppOperatorSource(dbInstance),
		Provision: seppProvision(dbInstance),
		N32:       n32,
	}, logger.SeppLog)

	sepp.RegisterMetrics(prometheus.DefaultRegisterer, m)
	server.SetSEPPClusterHandler(m.ClusterHandler())

	return m
}

func runSEPP(ctx context.Context, dbInstance *db.Database, m *sepp.Manager) {
	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicRoaming, db.TopicSEPPIdentity, db.TopicOperatorIdentity, db.TopicClusterMembers)
	defer stop()

	m.Run(ctx, wakeup)
}

func seppPartnersSource(dbInstance *db.Database) sepp.PartnersSource {
	return func(ctx context.Context) ([]sepp.PartnerConfig, error) {
		partners, err := dbInstance.ListRoamingPartners(ctx)
		if err != nil {
			return nil, err
		}

		out := make([]sepp.PartnerConfig, 0, len(partners))

		for _, p := range partners {
			if p.SEPPFQDN == nil {
				continue
			}

			cfg := sepp.PartnerConfig{
				ID:             p.ID,
				Name:           p.Name,
				FQDN:           *p.SEPPFQDN,
				N32cURI:        deref(p.SEPPN32cURI),
				N32fURI:        deref(p.SEPPN32fURI),
				TrustAnchorPEM: deref(p.SEPPTrustAnchorPEM),
				NRFAPIRoot:     deref(p.NRFAPIRoot),
			}

			for _, plmn := range p.PLMNs {
				cfg.PLMNs = append(cfg.PLMNs, wire.PlmnID{Mcc: plmn.Mcc, Mnc: plmn.Mnc})
			}

			out = append(out, cfg)
		}

		return out, nil
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func seppIdentitySource(dbInstance *db.Database) sepp.IdentitySource {
	return func(ctx context.Context) (*sepp.IdentityConfig, error) {
		seppID := dbInstance.AMFPointer()
		if seppID == 0 {
			return nil, nil
		}

		identity, err := dbInstance.GetSEPPIdentity(ctx, seppID)
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil
		}

		if err != nil {
			return nil, err
		}

		return &sepp.IdentityConfig{SEPPID: identity.SEPPID, FQDN: identity.FQDN, CertChainPEM: identity.CertChainPEM, KeyPEM: identity.KeyPEM}, nil
	}
}

func seppProvision(dbInstance *db.Database) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		seppID := dbInstance.AMFPointer()
		if seppID == 0 {
			return errors.New("this node has no SEPP ID yet: its cluster AMF Pointer is not allocated")
		}

		created, err := dbInstance.EnsureSEPPIdentity(ctx, seppID)
		if errors.Is(err, db.ErrMigrationPending) {
			return nil
		}

		if err != nil {
			return err
		}

		if created {
			logger.SeppLog.Info("Generated this node's SEPP certificate with the cluster SEPP CA", zap.Int("sepp_id", seppID))
		}

		return nil
	}
}

func seppOperatorSource(dbInstance *db.Database) sepp.OperatorSource {
	return func(ctx context.Context) (wire.PlmnID, error) {
		op, err := dbInstance.GetOperator(ctx)
		if err != nil {
			return wire.PlmnID{}, err
		}

		plmn := wire.PlmnID{Mcc: op.Mcc, Mnc: op.Mnc}
		if !plmn.Valid() {
			return wire.PlmnID{}, errors.New("the operator has no valid PLMN ID")
		}

		return plmn, nil
	}
}
