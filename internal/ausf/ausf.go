// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"regexp"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type AUSF struct {
	mu sync.RWMutex

	uePool              map[string]*AusfUeContext // Key: suci
	dbInstance          *db.Database
	servingNetworkRegex *regexp.Regexp
}

type AusfUeContext struct {
	Supi     string
	Kseaf    string
	XresStar string
	Rand     string
}

var ausf AUSF

func (a *AUSF) addUeContextToPool(suci string, ausfUeContext *AusfUeContext) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.uePool[suci] = ausfUeContext
}

func (a *AUSF) getUeContext(suci string) *AusfUeContext {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ausfUeContext, ok := a.uePool[suci]
	if !ok {
		return nil
	}

	logger.EllaLog.Error("current AUSF UE pool size", zap.Int("size", len(a.uePool)))

	return ausfUeContext
}

func (a *AUSF) isServingNetworkAuthorized(lookup string) bool {
	return a.servingNetworkRegex.MatchString(lookup)
}

func Start(dbInstance *db.Database) {
	ausf = AUSF{
		uePool:              make(map[string]*AusfUeContext),
		dbInstance:          dbInstance,
		servingNetworkRegex: regexp.MustCompile(`^5G:mnc[0-9]{3}\.mcc[0-9]{3}\.3gppnetwork\.org$`),
	}
}
