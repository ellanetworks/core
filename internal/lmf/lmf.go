// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/lppa"
	"github.com/ellanetworks/core/internal/lmf/nrppa"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
)

// LocationSource exposes the per-UE location a control-plane NF holds. The AMF
// (5G) and MME (4G) both satisfy it, so the LMF resolves a subscriber attached on
// either access.
type LocationSource interface {
	IsUERegistered(supi etsi.SUPI) bool
	GetUELocation(supi etsi.SUPI) (models.UserLocation, bool)
}

// LMF is the Location Management Function. It orchestrates positioning
// procedures and exposes a DetermineLocation method that the REST API
// calls to obtain a subscriber's current location.
type LMF struct {
	amf         *amf.AMF
	mme         *mme.MME
	db          *db.Database
	sessionMgr  *SessionManager
	nrppaClient *nrppa.Client
	lppaClient  *lppa.Client
}

// New creates an LMF instance that reads UE location from the AMF (5G) and the
// MME (4G).
func New(amfInstance *amf.AMF, mmeInstance *mme.MME, d *db.Database) *LMF {
	logger.LmfLog.Info("LMF initialized")

	return &LMF{
		amf:         amfInstance,
		mme:         mmeInstance,
		db:          d,
		sessionMgr:  NewSessionManager(d),
		nrppaClient: nrppa.New(amfInstance),
		lppaClient:  lppa.New(mmeInstance),
	}
}

// sources returns the location sources consulted in priority order: the AMF (5G)
// before the MME (4G). A subscriber is attached on at most one access, so the
// first source that owns the UE answers. A nil NF is skipped.
func (l *LMF) sources() []LocationSource {
	srcs := make([]LocationSource, 0, 2)

	if l.amf != nil {
		srcs = append(srcs, l.amf)
	}

	if l.mme != nil {
		srcs = append(srcs, l.mme)
	}

	return srcs
}

// isUERegistered reports whether any access has the UE registered.
func (l *LMF) isUERegistered(supi etsi.SUPI) bool {
	return anyRegistered(l.sources(), supi)
}

// getUELocation returns the location from the access that owns the UE.
func (l *LMF) getUELocation(supi etsi.SUPI) (models.UserLocation, bool) {
	return firstLocation(l.sources(), supi)
}

func anyRegistered(srcs []LocationSource, supi etsi.SUPI) bool {
	for _, s := range srcs {
		if s.IsUERegistered(supi) {
			return true
		}
	}

	return false
}

func firstLocation(srcs []LocationSource, supi etsi.SUPI) (models.UserLocation, bool) {
	for _, s := range srcs {
		if loc, ok := s.GetUELocation(supi); ok {
			return loc, true
		}
	}

	return models.UserLocation{}, false
}

// SessionManager returns the session manager for positioning session lifecycle.
func (l *LMF) SessionManager() *SessionManager {
	return l.sessionMgr
}
