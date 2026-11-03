// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

const lcsPrivacyFetchTimeout = 5 * time.Second

type HomeLCSPrivacy interface {
	LCSPrivacyData(ctx context.Context, t roaming.HomeTarget, supi string) (wire.LcsPrivacyData, error)
}

func (amf *AMF) LocationPrivacy(ctx context.Context, supi etsi.SUPI) error {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return nil
	}

	v, ok := ue.Visitor()
	if !ok {
		return nil
	}

	now := time.Now()

	if v.LCSPrivacy.Fresh(now) {
		return v.LCSPrivacy.Check(now)
	}

	p := amf.fetchLCSPrivacy(ctx, v, supi)

	if ctx.Err() == nil {
		ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.LCSPrivacy = &p })
	}

	return p.Check(now)
}

func (amf *AMF) fetchLCSPrivacy(ctx context.Context, v roaming.Visitor5G, supi etsi.SUPI) roaming.LCSPrivacy {
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPI(supi.String()), zap.String("partner", v.PartnerName))

	var (
		data wire.LcsPrivacyData
		err  error
	)

	if home, ok := amf.HomeNetwork.(HomeLCSPrivacy); ok {
		fetchCtx, cancel := context.WithTimeout(ctx, lcsPrivacyFetchTimeout)
		data, err = home.LCSPrivacyData(fetchCtx, v.Home, supi.String())

		cancel()
	} else {
		err = ErrNoHomeNetwork
	}

	p := roaming.NewLCSPrivacy(data, err, time.Now())

	if err != nil {
		log.Warn("could not read the roamer's LCS privacy profile from the home UDM; location is refused", zap.Error(err))
	} else {
		log.Info("read the roamer's LCS privacy profile from the home UDM",
			zap.Bool("om_in_vplmn_allowed", p.Check(time.Now()) == nil))
	}

	return p
}
