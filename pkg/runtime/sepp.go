// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"slices"

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
		Renew:     seppRenew(dbInstance),
		Cluster:   seppClusterSource(dbInstance),
		N32:       n32,
	}, logger.SeppLog)

	sepp.RegisterMetrics(prometheus.DefaultRegisterer, m)

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
			}

			for _, plmn := range p.PLMNs {
				cfg.PLMNs = append(cfg.PLMNs, wire.PlmnID{Mcc: plmn.Mcc, Mnc: plmn.Mnc})
			}

			if len(p.NRFAPIRoots) > 0 {
				cfg.NRFAPIRoots = make(map[wire.PlmnID]string, len(p.NRFAPIRoots))
				for _, r := range p.NRFAPIRoots {
					cfg.NRFAPIRoots[wire.PlmnID{Mcc: r.Mcc, Mnc: r.Mnc}] = r.APIRoot
				}
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

		return &sepp.IdentityConfig{SEPPID: identity.SEPPID, FQDN: identity.FQDN, CertChainPEM: identity.CertChainPEM, KeyPEM: identity.KeyPEM, Generated: identity.Source == db.SEPPIdentityGenerated}, nil
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

func seppRenew(dbInstance *db.Database) func(ctx context.Context, seppID int) error {
	return func(ctx context.Context, seppID int) error {
		identity, err := dbInstance.GetSEPPIdentity(ctx, seppID)
		if err != nil {
			return err
		}

		if identity.Source != db.SEPPIdentityGenerated {
			return errors.New("the SEPP certificate is imported: only the operator can replace it")
		}

		_, err = dbInstance.RenewGeneratedSEPPIdentity(ctx, seppID, identity.CertChainPEM)

		return err
	}
}

func seppClusterSource(dbInstance *db.Database) sepp.ClusterSource {
	return func(ctx context.Context) ([]int, error) {
		members, err := dbInstance.ListClusterMembers(ctx)
		if err != nil {
			return nil, err
		}

		identities, err := dbInstance.ListSEPPIdentities(ctx)
		if err != nil {
			return nil, err
		}

		ids := make([]int, 0, len(members))

		for _, member := range members {
			if slices.ContainsFunc(identities, func(i db.SEPPIdentity) bool { return i.SEPPID == member.AMFPointer }) {
				ids = append(ids, member.AMFPointer)
			}
		}

		return ids, nil
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
